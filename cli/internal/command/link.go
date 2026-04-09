package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/gitrepo"
	"lazyops-cli/internal/lazyyaml"
	"lazyops-cli/internal/repo"
	"lazyops-cli/internal/transport"
)

const linkSpinnerThreshold = time.Second

type linkArgs struct {
	Project      string
	Installation string
	Repo         string
	Branch       string
}

type githubRepoAccess struct {
	ID            int64
	Owner         string
	Name          string
	DefaultBranch string
}

type verifiedLinkTarget struct {
	ID     string
	Name   string
	Kind   string
	Status string
}

type projectRepoLinkRequest struct {
	InstallationID int64  `json:"installation_id"`
	RepoID         int64  `json:"repo_id"`
	TrackedBranch  string `json:"tracked_branch"`
}

func linkCommand() *Command {
	return &Command{
		Name:    "link",
		Summary: "Connect the local repo to a project and GitHub App installation.",
		Usage:   "lazyops link [--project <project-id-or-slug>] [--installation <id|account-login>] [--repo <owner/name>] [--branch <tracked-branch>]",
		Run:     withAuth(runLink),
	}
}

func runLink(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	linkArgs, err := parseLinkArgs(args)
	if err != nil {
		return err
	}

	repoRoot, err := repo.FindRepoRoot(".")
	if err != nil {
		if errors.Is(err, repo.ErrRepoRootNotFound) {
			return fmt.Errorf("could not find the repository root. next: run `lazyops link` from inside a git repository")
		}
		return fmt.Errorf("could not determine the repository root. next: verify the working tree is readable and retry `lazyops link`: %w", err)
	}

	gitMetadata, err := gitrepo.Load(repoRoot)
	if err != nil {
		if errors.Is(err, gitrepo.ErrOriginRemoteNotFound) {
			return fmt.Errorf("git origin remote is missing. next: set `origin` to a GitHub repository or rerun `lazyops link --repo <owner>/<name>`")
		}
		return fmt.Errorf("could not read git repo metadata. next: verify `.git/config` and `.git/HEAD`, or use `--repo` and `--branch` overrides: %w", err)
	}

	linkMetadata, err := lazyyaml.ReadLinkMetadata(repoRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("lazyops.yaml was not found at the repo root. next: run `lazyops init` before `lazyops link`")
		}
		return fmt.Errorf("could not read lazyops.yaml. next: repair the deploy contract or rerun `lazyops init`: %w", err)
	}

	projectSelector := strings.TrimSpace(linkArgs.Project)
	if projectSelector == "" {
		projectSelector = linkMetadata.ProjectSlug
	}

	repoOwner, repoName, err := resolveLinkRepo(linkArgs.Repo, gitMetadata)
	if err != nil {
		return err
	}

	projectsResponse, err := fetchProjects(ctx, runtime, credential)
	if err != nil {
		return err
	}
	project, err := selectProjectForLink(projectsResponse.Projects, projectSelector, credential)
	if err != nil {
		return err
	}

	bindingsResponse, err := fetchBindings(ctx, runtime, credential, project.ID)
	if err != nil {
		return err
	}
	binding, err := selectBindingForLink(bindingsResponse.Bindings, linkMetadata)
	if err != nil {
		return err
	}

	instancesResponse, err := fetchInstances(ctx, runtime, credential)
	if err != nil {
		return err
	}
	meshNetworksResponse, err := fetchMeshNetworks(ctx, runtime, credential)
	if err != nil {
		return err
	}
	clustersResponse, err := fetchClusters(ctx, runtime, credential)
	if err != nil {
		return err
	}
	discovery := initDiscovery{
		instances:    instancesResponse.Instances,
		meshNetworks: meshNetworksResponse.MeshNetworks,
		clusters:     clustersResponse.Clusters,
	}
	target, err := verifyLinkTarget(project, binding, discovery)
	if err != nil {
		return err
	}

	installationsResponse, err := fetchGitHubInstallations(ctx, runtime, credential)
	if err != nil {
		return err
	}
	installation, repoAccess, err := selectInstallationForRepo(installationsResponse.Installations, linkArgs.Installation, repoOwner, repoName)
	if err != nil {
		return err
	}

	trackedBranch, err := resolveTrackedBranch(linkArgs.Branch, gitMetadata.CurrentBranch, repoAccess.DefaultBranch, project.DefaultBranch)
	if err != nil {
		return err
	}

	printLinkReview(runtime, repoRoot, repoOwner, repoName, trackedBranch, project, installation, binding, target)

	linkResponse, err := createProjectRepoLink(ctx, runtime, credential, project.ID, projectRepoLinkRequest{
		InstallationID: installation.GitHubInstallationID,
		RepoID:         repoAccess.ID,
		TrackedBranch:  trackedBranch,
	})
	if err != nil {
		return err
	}

	runtime.Output.Success("repository linked")
	runtime.Output.Info("repo link: %s/%s -> %s on %s", linkResponse.RepoOwner, linkResponse.RepoName, project.Slug, linkResponse.TrackedBranch)
	if linkResponse.PreviewEnabled {
		runtime.Output.Info("preview environments: enabled")
	}

	return nil
}

func parseLinkArgs(args []string) (linkArgs, error) {
	flagSet := flag.NewFlagSet("link", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	project := flagSet.String("project", "", "project id or slug")
	installation := flagSet.String("installation", "", "GitHub App installation id or account login")
	repository := flagSet.String("repo", "", "repo override in owner/name form")
	branch := flagSet.String("branch", "", "tracked branch override")

	if err := flagSet.Parse(args); err != nil {
		return linkArgs{}, errors.New("invalid link flags. next: use `lazyops link [--project <project-id-or-slug>] [--installation <id|account-login>] [--repo <owner>/<name>] [--branch <tracked-branch>]`")
	}
	if flagSet.NArg() > 0 {
		return linkArgs{}, fmt.Errorf("unexpected link arguments: %s. next: use flags instead of positional arguments", strings.Join(flagSet.Args(), " "))
	}

	return linkArgs{
		Project:      strings.TrimSpace(*project),
		Installation: strings.TrimSpace(*installation),
		Repo:         strings.TrimSpace(*repository),
		Branch:       strings.TrimSpace(*branch),
	}, nil
}

func fetchGitHubInstallations(ctx context.Context, runtime *Runtime, credential credentials.Record) (contracts.GitHubInstallationsResponse, error) {
	body, err := json.Marshal(map[string]any{})
	if err != nil {
		return contracts.GitHubInstallationsResponse{}, err
	}

	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "POST",
		Path:   "/api/v1/github/app/installations/sync",
		Body:   body,
	}, credential))
	if err != nil {
		return contracts.GitHubInstallationsResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.GitHubInstallationsResponse{}, parseAPIError(response)
	}
	return contracts.DecodeGitHubInstallationsResponse(response.Body)
}

func resolveLinkRepo(repoOverride string, metadata gitrepo.Metadata) (string, string, error) {
	if strings.TrimSpace(repoOverride) != "" {
		return gitrepo.ParseRepoSlug(repoOverride)
	}

	if strings.TrimSpace(metadata.RepoOwner) == "" || strings.TrimSpace(metadata.RepoName) == "" {
		return "", "", errors.New("could not determine the local GitHub repo. next: set `origin` to a GitHub remote or use `--repo <owner>/<name>`")
	}

	return metadata.RepoOwner, metadata.RepoName, nil
}

func selectProjectForLink(projects []contracts.Project, selector string, credential credentials.Record) (contracts.Project, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return contracts.Project{}, errors.New("project selection is missing. next: rerun `lazyops link --project <project-id-or-slug>` or restore `project_slug` in lazyops.yaml")
	}

	for _, project := range projects {
		if selector != project.ID && selector != project.Slug && selector != project.Name {
			continue
		}
		return project, nil
	}

	return contracts.Project{}, fmt.Errorf("project %q was not found. next: rerun `lazyops link --project <project-id-or-slug>` with one of the projects available to this account", selector)
}

func selectBindingForLink(bindings []contracts.DeploymentBinding, metadata lazyyaml.LinkMetadata) (contracts.DeploymentBinding, error) {
	for _, binding := range bindings {
		if binding.RuntimeMode != string(metadata.RuntimeMode) {
			continue
		}
		if binding.TargetRef != metadata.TargetRef {
			continue
		}
		return binding, nil
	}

	return contracts.DeploymentBinding{}, fmt.Errorf("no deployment binding matches target_ref %q in runtime mode %q. next: rerun `lazyops init` to create or reuse a compatible binding", metadata.TargetRef, metadata.RuntimeMode)
}

func verifyLinkTarget(project contracts.Project, binding contracts.DeploymentBinding, discovery initDiscovery) (verifiedLinkTarget, error) {
	target, ok := resolveTargetForBinding(binding, discovery)
	if !ok {
		return verifiedLinkTarget{}, fmt.Errorf("deployment binding %q points to a target that no longer exists. next: rerun `lazyops init` and choose a valid target", binding.Name)
	}
	if !isLinkableTargetStatus(target.Status) {
		return verifiedLinkTarget{}, fmt.Errorf("%s target %q is not online or registered. next: bring the target online, wait for registration, or choose a different binding", target.Kind, target.Name)
	}

	return target, nil
}

func resolveTargetForBinding(binding contracts.DeploymentBinding, discovery initDiscovery) (verifiedLinkTarget, bool) {
	switch binding.TargetKind {
	case "instance":
		for _, instance := range discovery.instances {
			if instance.ID == binding.TargetID {
				return verifiedLinkTarget{
					ID:     instance.ID,
					Name:   instance.Name,
					Kind:   "instance",
					Status: instance.Status,
				}, true
			}
		}
	case "mesh":
		for _, network := range discovery.meshNetworks {
			if network.ID == binding.TargetID {
				return verifiedLinkTarget{
					ID:     network.ID,
					Name:   network.Name,
					Kind:   "mesh",
					Status: network.Status,
				}, true
			}
		}
	case "cluster":
		for _, cluster := range discovery.clusters {
			if cluster.ID == binding.TargetID {
				return verifiedLinkTarget{
					ID:     cluster.ID,
					Name:   cluster.Name,
					Kind:   "cluster",
					Status: cluster.Status,
				}, true
			}
		}
	}

	return verifiedLinkTarget{}, false
}

func isLinkableTargetStatus(status string) bool {
	return strings.EqualFold(status, "online") ||
		strings.EqualFold(status, "registered") ||
		strings.EqualFold(status, "available")
}

func selectInstallationForRepo(installations []contracts.GitHubInstallation, selector string, repoOwner string, repoName string) (contracts.GitHubInstallation, githubRepoAccess, error) {
	if strings.TrimSpace(selector) != "" {
		for _, installation := range installations {
			if !matchesInstallationSelector(installation, selector) {
				continue
			}
			repoAccess, ok, err := repoAccessForInstallation(installation, repoOwner, repoName)
			if err != nil {
				return contracts.GitHubInstallation{}, githubRepoAccess{}, err
			}
			if !ok {
				return contracts.GitHubInstallation{}, githubRepoAccess{}, fmt.Errorf("GitHub App installation %q does not grant access to %s/%s. next: install the GitHub App on that repo or pick a different installation", selector, repoOwner, repoName)
			}
			return installation, repoAccess, nil
		}

		return contracts.GitHubInstallation{}, githubRepoAccess{}, fmt.Errorf("GitHub App installation %q was not found. next: rerun `lazyops link --installation <id|account-login>` with an installation available to this account", selector)
	}

	type match struct {
		installation contracts.GitHubInstallation
		repo         githubRepoAccess
	}
	matches := []match{}
	for _, installation := range installations {
		repoAccess, ok, err := repoAccessForInstallation(installation, repoOwner, repoName)
		if err != nil {
			return contracts.GitHubInstallation{}, githubRepoAccess{}, err
		}
		if !ok {
			continue
		}
		matches = append(matches, match{installation: installation, repo: repoAccess})
	}

	switch len(matches) {
	case 0:
		return contracts.GitHubInstallation{}, githubRepoAccess{}, fmt.Errorf("no GitHub App installation grants access to %s/%s. next: install the LazyOps GitHub App on that repo or use `lazyops link --installation <id|account-login>` after syncing access", repoOwner, repoName)
	case 1:
		return matches[0].installation, matches[0].repo, nil
	default:
		return contracts.GitHubInstallation{}, githubRepoAccess{}, fmt.Errorf("multiple GitHub App installations grant access to %s/%s. next: rerun `lazyops link --installation <id|account-login>` to choose one", repoOwner, repoName)
	}
}

func matchesInstallationSelector(installation contracts.GitHubInstallation, selector string) bool {
	normalized := strings.TrimSpace(selector)
	if normalized == "" {
		return false
	}

	if installation.ID == normalized || strings.EqualFold(installation.AccountLogin, normalized) {
		return true
	}
	if installation.GitHubInstallationID > 0 && strconv.FormatInt(installation.GitHubInstallationID, 10) == normalized {
		return true
	}
	return false
}

func repoAccessForInstallation(installation contracts.GitHubInstallation, repoOwner string, repoName string) (githubRepoAccess, bool, error) {
	repositories, err := repositoriesFromInstallationScope(installation.ScopeJSON)
	if err != nil {
		return githubRepoAccess{}, false, fmt.Errorf("GitHub App installation %q has an invalid repo scope. next: resync installations and retry `lazyops link`: %w", installation.AccountLogin, err)
	}

	for _, repository := range repositories {
		if strings.EqualFold(repository.Owner, repoOwner) && strings.EqualFold(repository.Name, repoName) {
			return repository, true, nil
		}
	}

	return githubRepoAccess{}, false, nil
}

func repositoriesFromInstallationScope(scope map[string]any) ([]githubRepoAccess, error) {
	rawRepositories, ok := scope["repositories"]
	if !ok {
		return nil, errors.New("scope_json.repositories is missing. next: verify the GitHub App installation includes repository scope")
	}

	items, ok := rawRepositories.([]any)
	if !ok {
		if typed, typedOK := rawRepositories.([]map[string]any); typedOK {
			items = make([]any, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		} else {
			return nil, errors.New("scope_json.repositories must be an array. next: verify the GitHub App installation scope_json.repositories is a list")
		}
	}

	repositories := make([]githubRepoAccess, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("scope_json.repositories entries must be objects. next: verify the GitHub App installation scope_json.repositories contains valid repo objects")
		}

		repoID, err := coerceInt64(entry["id"])
		if err != nil {
			return nil, fmt.Errorf("scope_json.repositories[].id: %w", err)
		}

		name, _ := entry["name"].(string)
		owner, _ := entry["owner"].(string)
		if strings.TrimSpace(owner) == "" {
			owner, _ = entry["owner_login"].(string)
		}
		defaultBranch, _ := entry["default_branch"].(string)
		if strings.TrimSpace(name) == "" || strings.TrimSpace(owner) == "" {
			return nil, errors.New("scope_json.repositories entries must include owner and name. next: verify the GitHub App installation scope includes full repository objects")
		}

		repositories = append(repositories, githubRepoAccess{
			ID:            repoID,
			Owner:         owner,
			Name:          name,
			DefaultBranch: defaultBranch,
		})
	}

	return repositories, nil
}

func coerceInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float64:
		return int64(typed), nil
	case json.Number:
		return typed.Int64()
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported numeric value %T", value)
	}
}

func resolveTrackedBranch(explicit string, currentBranch string, repoDefaultBranch string, projectDefaultBranch string) (string, error) {
	for _, candidate := range []string{
		strings.TrimSpace(explicit),
		strings.TrimSpace(currentBranch),
		strings.TrimSpace(repoDefaultBranch),
		strings.TrimSpace(projectDefaultBranch),
	} {
		if candidate == "" {
			continue
		}
		return candidate, nil
	}

	return "", errors.New("tracked branch could not be determined. next: checkout a branch locally or rerun `lazyops link --branch <tracked-branch>`")
}

func printLinkReview(runtime *Runtime, repoRoot string, repoOwner string, repoName string, branch string, project contracts.Project, installation contracts.GitHubInstallation, binding contracts.DeploymentBinding, target verifiedLinkTarget) {
	runtime.Output.Success("repo link review ready")
	runtime.Output.Info("transport mode: %s", runtime.Transport.Mode())
	runtime.Output.Info("repo root: %s", repoRoot)
	runtime.Output.Info("local repo: %s/%s", repoOwner, repoName)
	runtime.Output.Info("tracked branch: %s", branch)
	runtime.Output.Info("selected project: %s (%s)", project.Name, project.Slug)
	runtime.Output.Info("github app installation: %s (%d)", installation.AccountLogin, installation.GitHubInstallationID)
	runtime.Output.Info("verified binding: %s -> %s (%s, %s)", binding.Name, binding.TargetRef, binding.RuntimeMode, binding.TargetKind)
	runtime.Output.Info("verified target: %s %s [%s]", target.Kind, target.Name, target.Status)
}

func createProjectRepoLink(ctx context.Context, runtime *Runtime, credential credentials.Record, projectID string, payload projectRepoLinkRequest) (contracts.ProjectRepoLink, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return contracts.ProjectRepoLink{}, err
	}

	response, err := doWithDelayedSpinner(ctx, runtime, linkSpinnerThreshold, "linking repository to project", func(ctx context.Context) (transport.Response, error) {
		return runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
			Method: "POST",
			Path:   fmt.Sprintf("/api/v1/projects/%s/repo-link", projectID),
			Body:   body,
		}, credential))
	})
	if err != nil {
		return contracts.ProjectRepoLink{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.ProjectRepoLink{}, parseAPIError(response)
	}

	return contracts.DecodeProjectRepoLink(response.Body)
}
