package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	doctorreport "lazyops-cli/internal/doctor"
	"lazyops-cli/internal/gitrepo"
	"lazyops-cli/internal/lazyyaml"
	"lazyops-cli/internal/repo"
	"lazyops-cli/internal/transport"
)

type doctorPreviewResponse struct {
	Checks []doctorPreviewCheck `json:"checks"`
}

type doctorPreviewCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Summary  string `json:"summary"`
	NextStep string `json:"next_step"`
}

type repoInstallationAccess struct {
	installation contracts.GitHubInstallation
	repo         githubRepoAccess
}

func doctorCommand() *Command {
	return &Command{
		Name:    "doctor",
		Summary: "Validate local onboarding and deploy contract health.",
		Usage:   "lazyops doctor",
		Run:     runDoctor,
	}
}

func runDoctor(ctx context.Context, runtime *Runtime, args []string) error {
	if err := parseDoctorArgs(args); err != nil {
		return err
	}

	report := doctorreport.Report{}

	repoRoot, repoRootErr := repo.FindRepoRoot(".")
	if repoRootErr == nil {
		report.RepoRoot = repoRoot
	}

	metadata, metadataErr := readDoctorMetadata(repoRoot, repoRootErr)
	if metadataErr == nil {
		report.ProjectSlug = metadata.ProjectSlug
	}
	linkFieldsValid := metadataErr == nil && metadata.ValidateLinkFields() == nil
	contractValid := metadataErr == nil && metadata.ValidateDoctorContract() == nil

	report.Add(checkDoctorAuth(ctx, runtime))
	authCheck := report.Checks[len(report.Checks)-1]

	report.Add(checkDoctorLazyopsYAML(repoRootErr, metadata, metadataErr))

	var credentialRecord transportCredential
	if authCheck.Status == doctorreport.StatusPass {
		credential, _ := requireAuth(ctx, runtime)
		credentialRecord = transportCredential{
			record: credential,
			ok:     true,
		}
	}

	var gitMetadata gitrepo.Metadata
	var gitMetadataErr error
	if repoRootErr == nil {
		gitMetadata, gitMetadataErr = gitrepo.Load(repoRoot)
	}

	var projects projectsSelection
	if credentialRecord.ok && linkFieldsValid {
		projects = selectDoctorProject(ctx, runtime, credentialRecord.record, metadata.ProjectSlug)
	}

	report.Add(checkDoctorRepoLink(ctx, runtime, credentialRecord, repoRootErr, gitMetadata, gitMetadataErr))
	report.Add(checkDoctorBinding(ctx, runtime, credentialRecord, metadata, metadataErr, linkFieldsValid, projects))
	report.Add(checkDoctorDependencies(repoRootErr, repoRoot, metadata, metadataErr, contractValid))
	report.Add(checkDoctorWebhook(ctx, runtime, credentialRecord, linkFieldsValid, projects))

	printDoctorReport(runtime, report)
	if report.HasFailures() {
		return fmt.Errorf("doctor found %d failing checks. next: fix the reported items and rerun `lazyops doctor`", report.FailureCount())
	}

	return nil
}

type transportCredential struct {
	record credentials.Record
	ok     bool
}

type projectsSelection struct {
	selected *contracts.Project
	err      error
}

func parseDoctorArgs(args []string) error {
	flagSet := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	if err := flagSet.Parse(args); err != nil {
		return errors.New("invalid doctor flags. next: use `lazyops doctor`")
	}
	if flagSet.NArg() > 0 {
		return fmt.Errorf("unexpected doctor arguments: %s. next: use `lazyops doctor`", strings.Join(flagSet.Args(), " "))
	}
	return nil
}

func readDoctorMetadata(repoRoot string, repoRootErr error) (lazyyaml.DoctorMetadata, error) {
	if repoRootErr != nil {
		return lazyyaml.DoctorMetadata{}, repoRootErr
	}
	return lazyyaml.ReadDoctorMetadata(repoRoot)
}

func checkDoctorAuth(ctx context.Context, runtime *Runtime) doctorreport.Check {
	credential, err := requireAuth(ctx, runtime)
	if err != nil {
		summary, nextStep := splitNextStep(err.Error())
		return failDoctorCheck("auth", summary, nextStep)
	}

	projectsResponse, err := fetchProjects(ctx, runtime, credential)
	if err != nil {
		summary, nextStep := splitNextStep(err.Error())
		if nextStep == "" {
			nextStep = "rerun `lazyops login` to refresh the CLI PAT and retry `lazyops doctor`"
		}
		return failDoctorCheck("auth", summary, nextStep)
	}

	displayName := strings.TrimSpace(credential.DisplayName)
	if displayName == "" {
		displayName = "CLI user"
	}

	return passDoctorCheck("auth", fmt.Sprintf("CLI session is active for %s and can reach %d project(s)", displayName, len(projectsResponse.Projects)))
}

func checkDoctorLazyopsYAML(repoRootErr error, metadata lazyyaml.DoctorMetadata, metadataErr error) doctorreport.Check {
	if repoRootErr != nil {
		if errors.Is(repoRootErr, repo.ErrRepoRootNotFound) {
			return failDoctorCheck("lazyops_yaml", "repository root was not found", "run `lazyops doctor` from inside a git repository that contains lazyops.yaml")
		}
		return failDoctorCheck("lazyops_yaml", "repository root could not be resolved", "verify the working tree is readable and rerun `lazyops doctor`")
	}
	if metadataErr != nil {
		if errors.Is(metadataErr, os.ErrNotExist) {
			return failDoctorCheck("lazyops_yaml", "lazyops.yaml is missing at the repository root", "run `lazyops init` to generate the deploy contract before rerunning `lazyops doctor`")
		}
		summary, nextStep := splitNextStep(metadataErr.Error())
		if nextStep == "" {
			nextStep = "repair lazyops.yaml or rerun `lazyops init` before rerunning `lazyops doctor`"
		}
		return failDoctorCheck("lazyops_yaml", summary, nextStep)
	}
	if err := metadata.ValidateDoctorContract(); err != nil {
		summary, nextStep := splitNextStep(err.Error())
		if nextStep == "" {
			nextStep = "repair lazyops.yaml or rerun `lazyops init` so the deploy contract is complete"
		}
		return failDoctorCheck("lazyops_yaml", summary, nextStep)
	}

	return passDoctorCheck(
		"lazyops_yaml",
		fmt.Sprintf(
			"lazyops.yaml declares project %s, runtime %s, target_ref %s, and %d service(s)",
			metadata.ProjectSlug,
			metadata.RuntimeMode,
			metadata.TargetRef,
			len(metadata.Services),
		),
	)
}

func checkDoctorRepoLink(
	ctx context.Context,
	runtime *Runtime,
	credential transportCredential,
	repoRootErr error,
	gitMetadata gitrepo.Metadata,
	gitMetadataErr error,
) doctorreport.Check {
	if repoRootErr != nil {
		return failDoctorCheck("repo_link", "repository root was not found", "run `lazyops doctor` from inside the local git repository you want to link")
	}
	if gitMetadataErr != nil {
		summary, nextStep := splitNextStep(gitMetadataErr.Error())
		if nextStep == "" {
			nextStep = "repair the local git metadata or rerun `lazyops link --repo <owner>/<name>`"
		}
		return failDoctorCheck("repo_link", summary, nextStep)
	}
	if !credential.ok {
		return warnDoctorCheck("repo_link", "GitHub App repo access was not checked because CLI auth is unhealthy", "run `lazyops login` first, then rerun `lazyops doctor` to verify repo link readiness")
	}

	installationsResponse, err := fetchGitHubInstallations(ctx, runtime, credential.record)
	if err != nil {
		summary, nextStep := splitNextStep(err.Error())
		if nextStep == "" {
			nextStep = "retry `lazyops doctor` or rerun `lazyops link` after GitHub App sync succeeds"
		}
		return failDoctorCheck("repo_link", summary, nextStep)
	}

	matches, err := accessibleInstallationsForRepo(installationsResponse.Installations, gitMetadata.RepoOwner, gitMetadata.RepoName)
	if err != nil {
		summary, nextStep := splitNextStep(err.Error())
		if nextStep == "" {
			nextStep = "repair the GitHub App installation scopes and rerun `lazyops link`"
		}
		return failDoctorCheck("repo_link", summary, nextStep)
	}

	switch len(matches) {
	case 0:
		return failDoctorCheck(
			"repo_link",
			fmt.Sprintf("no GitHub App installation grants access to %s/%s", gitMetadata.RepoOwner, gitMetadata.RepoName),
			"install the LazyOps GitHub App on the repo or rerun `lazyops link --installation <id|account-login>` after syncing access",
		)
	case 1:
		match := matches[0]
		return passDoctorCheck(
			"repo_link",
			fmt.Sprintf(
				"GitHub App installation %s (%d) grants access to %s/%s on branch %s",
				match.installation.AccountLogin,
				match.installation.GitHubInstallationID,
				gitMetadata.RepoOwner,
				gitMetadata.RepoName,
				fallbackDoctorBranch(gitMetadata.CurrentBranch, match.repo.DefaultBranch),
			),
		)
	default:
		accountLogins := make([]string, 0, len(matches))
		for _, match := range matches {
			accountLogins = append(accountLogins, match.installation.AccountLogin)
		}
		sort.Strings(accountLogins)
		return warnDoctorCheck(
			"repo_link",
			fmt.Sprintf("multiple GitHub App installations grant access to %s/%s (%s)", gitMetadata.RepoOwner, gitMetadata.RepoName, strings.Join(accountLogins, ", ")),
			"rerun `lazyops link --installation <id|account-login>` to pin the intended installation before deploying",
		)
	}
}

func checkDoctorBinding(
	ctx context.Context,
	runtime *Runtime,
	credential transportCredential,
	metadata lazyyaml.DoctorMetadata,
	metadataErr error,
	linkFieldsValid bool,
	projects projectsSelection,
) doctorreport.Check {
	if metadataErr != nil || !linkFieldsValid {
		return warnDoctorCheck("binding", "deployment binding could not be checked because lazyops.yaml is incomplete", "repair lazyops.yaml or rerun `lazyops init`, then rerun `lazyops doctor`")
	}
	if !credential.ok {
		return warnDoctorCheck("binding", "deployment binding could not be checked because CLI auth is unhealthy", "run `lazyops login` first, then rerun `lazyops doctor`")
	}
	if projects.err != nil {
		summary, nextStep := splitNextStep(projects.err.Error())
		if nextStep == "" {
			nextStep = "repair project_slug in lazyops.yaml or rerun `lazyops init --project <project-id-or-slug>`"
		}
		return failDoctorCheck("binding", summary, nextStep)
	}
	if projects.selected == nil {
		return failDoctorCheck("binding", "project selection is unavailable for this deploy contract", "set project_slug in lazyops.yaml or rerun `lazyops init`")
	}

	bindingsResponse, err := fetchBindings(ctx, runtime, credential.record, projects.selected.ID)
	if err != nil {
		summary, nextStep := splitNextStep(err.Error())
		if nextStep == "" {
			nextStep = "retry `lazyops doctor` after the deployment binding list endpoint is healthy"
		}
		return failDoctorCheck("binding", summary, nextStep)
	}

	binding, err := selectBindingForLink(bindingsResponse.Bindings, metadata.LinkMetadata())
	if err != nil {
		summary, nextStep := splitNextStep(err.Error())
		if nextStep == "" {
			nextStep = "rerun `lazyops init` to create or reuse a compatible deployment binding"
		}
		return failDoctorCheck("binding", summary, nextStep)
	}

	discovery, err := fetchTargetDiscovery(ctx, runtime, credential.record)
	if err != nil {
		summary, nextStep := splitNextStep(err.Error())
		if nextStep == "" {
			nextStep = "retry `lazyops doctor` after the target discovery endpoints are healthy"
		}
		return failDoctorCheck("binding", summary, nextStep)
	}

	target, ok := resolveTargetForBinding(binding, discovery)
	if !ok {
		return failDoctorCheck("binding", fmt.Sprintf("deployment binding %s points to a target that no longer exists", binding.Name), "rerun `lazyops init` and choose a valid target before deploying")
	}
	if !isLinkableTargetStatus(target.Status) {
		return warnDoctorCheck("binding", fmt.Sprintf("deployment binding %s targets %s %s, but the target status is %s", binding.Name, target.Kind, target.Name, target.Status), "bring the target back online or choose a different target before the next deploy")
	}

	return passDoctorCheck("binding", fmt.Sprintf("deployment binding %s targets %s %s with status %s", binding.Name, target.Kind, target.Name, target.Status))
}

func checkDoctorDependencies(
	repoRootErr error,
	repoRoot string,
	metadata lazyyaml.DoctorMetadata,
	metadataErr error,
	contractValid bool,
) doctorreport.Check {
	if repoRootErr != nil {
		return warnDoctorCheck("dependency_declarations", "service and dependency declarations were not checked because the repository root is unavailable", "run `lazyops doctor` from inside the repository root")
	}
	if metadataErr != nil || !contractValid {
		return warnDoctorCheck("dependency_declarations", "service and dependency declarations were not checked because lazyops.yaml is incomplete", "repair lazyops.yaml or rerun `lazyops init`, then rerun `lazyops doctor`")
	}
	if err := metadata.ValidateDependencyDeclarations(); err != nil {
		summary, nextStep := splitNextStep(err.Error())
		if nextStep == "" {
			nextStep = "repair dependency_bindings in lazyops.yaml or rerun `lazyops init`"
		}
		return failDoctorCheck("dependency_declarations", summary, nextStep)
	}

	scanResult, err := repo.Scan(repoRoot)
	if err != nil {
		return failDoctorCheck("dependency_declarations", "local services could not be scanned", "verify the repository is readable and rerun `lazyops doctor`")
	}
	detectionResult, err := repo.DetectServices(scanResult)
	if err != nil {
		return failDoctorCheck("dependency_declarations", "local services could not be inferred from the repository", "fix the repo service layout or rerun `lazyops init` to refresh lazyops.yaml")
	}

	declaredByPath := make(map[string]lazyyaml.DoctorService, len(metadata.Services))
	for _, service := range metadata.Services {
		declaredByPath[service.Path] = service
	}

	missingDeclarations := []string{}
	detectedByPath := make(map[string]struct{}, len(detectionResult.Candidates))
	for _, candidate := range detectionResult.Candidates {
		detectedByPath[candidate.Path] = struct{}{}
		if _, ok := declaredByPath[candidate.Path]; ok {
			continue
		}
		missingDeclarations = append(missingDeclarations, candidate.Path)
	}
	if len(missingDeclarations) > 0 {
		sort.Strings(missingDeclarations)
		return failDoctorCheck(
			"dependency_declarations",
			fmt.Sprintf("lazyops.yaml is missing service declarations for %s", strings.Join(missingDeclarations, ", ")),
			"rerun `lazyops init` so the deploy contract includes every detected service",
		)
	}

	staleDeclarations := []string{}
	for _, service := range metadata.Services {
		if _, ok := detectedByPath[service.Path]; ok {
			continue
		}
		staleDeclarations = append(staleDeclarations, service.Path)
	}
	if len(staleDeclarations) > 0 {
		sort.Strings(staleDeclarations)
		return warnDoctorCheck(
			"dependency_declarations",
			fmt.Sprintf("lazyops.yaml still references service paths that are no longer detected: %s", strings.Join(staleDeclarations, ", ")),
			"refresh the deploy contract with `lazyops init --write --overwrite` after reviewing the current repo layout",
		)
	}

	return passDoctorCheck(
		"dependency_declarations",
		fmt.Sprintf("%d detected service(s) are declared and %d dependency binding(s) validate cleanly", len(detectionResult.Candidates), len(metadata.DependencyBindings)),
	)
}

func checkDoctorWebhook(
	ctx context.Context,
	runtime *Runtime,
	credential transportCredential,
	linkFieldsValid bool,
	projects projectsSelection,
) doctorreport.Check {
	if !linkFieldsValid {
		return warnDoctorCheck("webhook_health", "webhook health was not checked because lazyops.yaml is missing project or binding metadata", "repair lazyops.yaml or rerun `lazyops init` before rerunning `lazyops doctor`")
	}
	if !credential.ok {
		return warnDoctorCheck("webhook_health", "webhook health was not checked because CLI auth is unhealthy", "run `lazyops login` first, then rerun `lazyops doctor`")
	}
	if projects.err != nil || projects.selected == nil {
		return warnDoctorCheck("webhook_health", "webhook health was not checked because the project could not be resolved from lazyops.yaml", "repair project_slug in lazyops.yaml or rerun `lazyops init --project <project-id-or-slug>`")
	}

	preview, err := fetchDoctorPreview(ctx, runtime, credential.record, projects.selected.ID)
	if err != nil {
		return warnDoctorCheck("webhook_health", "webhook health preview is unavailable in the current transport mode", "retry `lazyops doctor` after the webhook health contract is exposed by the control plane")
	}

	for _, check := range preview.Checks {
		if strings.TrimSpace(check.Name) != "webhook_health" {
			continue
		}

		status := doctorreport.NormalizeStatus(check.Status)
		summary := strings.TrimSpace(check.Summary)
		if summary == "" {
			summary = "webhook health preview responded without a summary"
		}
		nextStep := strings.TrimSpace(check.NextStep)
		switch status {
		case doctorreport.StatusPass:
			return passDoctorCheck("webhook_health", summary)
		case doctorreport.StatusFail:
			if nextStep == "" {
				nextStep = "repair the control-plane webhook integration before the next deploy"
			}
			return failDoctorCheck("webhook_health", summary, nextStep)
		default:
			if nextStep == "" {
				nextStep = "review the control-plane webhook integration before the next deploy"
			}
			return warnDoctorCheck("webhook_health", summary, nextStep)
		}
	}

	return warnDoctorCheck("webhook_health", "webhook health preview did not return a dedicated webhook check", "retry `lazyops doctor` after the control-plane doctor contract exposes webhook health")
}

func fetchTargetDiscovery(ctx context.Context, runtime *Runtime, credential credentials.Record) (initDiscovery, error) {
	instancesResponse, err := fetchInstances(ctx, runtime, credential)
	if err != nil {
		return initDiscovery{}, err
	}
	meshNetworksResponse, err := fetchMeshNetworks(ctx, runtime, credential)
	if err != nil {
		return initDiscovery{}, err
	}
	clustersResponse, err := fetchClusters(ctx, runtime, credential)
	if err != nil {
		return initDiscovery{}, err
	}

	return initDiscovery{
		instances:    instancesResponse.Instances,
		meshNetworks: meshNetworksResponse.MeshNetworks,
		clusters:     clustersResponse.Clusters,
	}, nil
}

func fetchDoctorPreview(ctx context.Context, runtime *Runtime, credential credentials.Record, projectID string) (doctorPreviewResponse, error) {
	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "GET",
		Path:   "/mock/v1/doctor",
		Query: map[string]string{
			"project": projectID,
		},
	}, credential))
	if err != nil {
		return doctorPreviewResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return doctorPreviewResponse{}, parseAPIError(response)
	}

	var preview doctorPreviewResponse
	if err := json.Unmarshal(response.Body, &preview); err != nil {
		return doctorPreviewResponse{}, fmt.Errorf("could not decode doctor preview response. next: verify the backend doctor preview contract returns valid JSON: %w", err)
	}
	return preview, nil
}

func selectDoctorProject(ctx context.Context, runtime *Runtime, credential credentials.Record, selector string) projectsSelection {
	projectsResponse, err := fetchProjects(ctx, runtime, credential)
	if err != nil {
		return projectsSelection{err: err}
	}

	selectedProject, err := selectProjectForLink(projectsResponse.Projects, selector, credential)
	if err != nil {
		return projectsSelection{err: err}
	}
	return projectsSelection{selected: &selectedProject}
}

func accessibleInstallationsForRepo(installations []contracts.GitHubInstallation, repoOwner string, repoName string) ([]repoInstallationAccess, error) {
	matches := make([]repoInstallationAccess, 0, len(installations))
	for _, installation := range installations {
		repoAccess, ok, err := repoAccessForInstallation(installation, repoOwner, repoName)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		matches = append(matches, repoInstallationAccess{
			installation: installation,
			repo:         repoAccess,
		})
	}
	return matches, nil
}

func printDoctorReport(runtime *Runtime, report doctorreport.Report) {
	pass, warn, fail := report.Counts()

	runtime.Output.Print("doctor report")
	if strings.TrimSpace(report.RepoRoot) != "" {
		runtime.Output.Info("repo: %s", report.RepoRoot)
	}
	if strings.TrimSpace(report.ProjectSlug) != "" {
		runtime.Output.Info("project: %s", report.ProjectSlug)
	}

	for _, check := range report.Checks {
		switch check.Status {
		case doctorreport.StatusPass:
			runtime.Output.Success("pass %s: %s", check.Name, check.Summary)
		case doctorreport.StatusFail:
			runtime.Output.Error("fail %s: %s", check.Name, check.Summary)
		default:
			runtime.Output.Warn("warn %s: %s", check.Name, check.Summary)
		}
		if strings.TrimSpace(check.NextStep) != "" {
			runtime.Output.Info("next: %s", check.NextStep)
		}
	}

	runtime.Output.Info("summary: %d pass, %d warn, %d fail", pass, warn, fail)
}

func passDoctorCheck(name string, summary string) doctorreport.Check {
	return doctorreport.Check{
		Name:    name,
		Status:  doctorreport.StatusPass,
		Summary: strings.TrimSpace(summary),
	}
}

func warnDoctorCheck(name string, summary string, nextStep string) doctorreport.Check {
	return doctorreport.Check{
		Name:     name,
		Status:   doctorreport.StatusWarn,
		Summary:  strings.TrimSpace(summary),
		NextStep: strings.TrimSpace(nextStep),
	}
}

func failDoctorCheck(name string, summary string, nextStep string) doctorreport.Check {
	return doctorreport.Check{
		Name:     name,
		Status:   doctorreport.StatusFail,
		Summary:  strings.TrimSpace(summary),
		NextStep: strings.TrimSpace(nextStep),
	}
}

func splitNextStep(message string) (string, string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return "", ""
	}

	lower := strings.ToLower(trimmed)
	index := strings.Index(lower, " next: ")
	if index == -1 {
		return trimmed, ""
	}

	summary := strings.TrimSpace(trimmed[:index])
	nextStep := strings.TrimSpace(trimmed[index+len(" next: "):])
	return summary, nextStep
}

func fallbackDoctorBranch(branch string, fallback string) string {
	if strings.TrimSpace(branch) != "" {
		return branch
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "unknown"
}
