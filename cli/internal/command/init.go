package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"lazyops-cli/internal/contracts"
	"lazyops-cli/internal/credentials"
	"lazyops-cli/internal/initplan"
	"lazyops-cli/internal/lazyyaml"
	"lazyops-cli/internal/repo"
	"lazyops-cli/internal/transport"
)

type initArgs struct {
	Project             string
	RuntimeMode         initplan.RuntimeMode
	Target              string
	Binding             string
	CreateBinding       bool
	BindingName         string
	Write               bool
	Overwrite           bool
	MagicDomainProvider string
	ScaleToZero         bool
}

func runInit(ctx context.Context, runtime *Runtime, args []string, credential credentials.Record) error {
	initArgs, err := parseInitArgs(args)
	if err != nil {
		return err
	}

	scanResult, err := repo.Scan(".")
	if err != nil {
		if errors.Is(err, repo.ErrRepoRootNotFound) {
			return fmt.Errorf("could not find the repository root. next: run `lazyops init` from inside a git repository")
		}
		return fmt.Errorf("could not scan the local repository. next: verify the working tree is readable and retry `lazyops init`: %w", err)
	}

	detectionResult, err := repo.DetectServices(scanResult)
	if err != nil {
		return fmt.Errorf("could not turn scan results into service candidates. next: fix the detected service layout and retry `lazyops init`: %w", err)
	}

	plan, err := initplan.Build(scanResult, detectionResult)
	if err != nil {
		return fmt.Errorf("could not build the init plan review. next: fix the detected service layout and retry `lazyops init`: %w", err)
	}

	discovery, err := fetchInitDiscovery(ctx, runtime, credential)
	if err != nil {
		return err
	}

	plan, err = initplan.ApplyDiscovery(plan, discovery.projects, discovery.instances, discovery.meshNetworks, discovery.clusters, initplan.SelectionInput{
		Project:     initArgs.Project,
		RuntimeMode: initArgs.RuntimeMode,
		Target:      initArgs.Target,
	})
	if err != nil {
		return err
	}

	if plan.SelectedProject != nil {
		bindingsResponse, err := fetchBindings(ctx, runtime, credential, plan.SelectedProject.ID)
		if err != nil {
			return err
		}

		plan, err = initplan.ApplyBindings(plan, bindingsResponse.Bindings, initplan.BindingSelectionInput{
			Binding:     initArgs.Binding,
			Create:      initArgs.CreateBinding,
			BindingName: initArgs.BindingName,
		})
		if err != nil {
			return err
		}

		if initArgs.CreateBinding && plan.SelectedBinding != nil && strings.TrimSpace(plan.SelectedBinding.ID) == "" {
			createdBinding, err := createDeploymentBinding(ctx, runtime, credential, plan.SelectedProject.ID, *plan.SelectedBinding)
			if err != nil {
				return err
			}
			plan, err = initplan.ApplyBindings(plan, append(bindingsResponse.Bindings, createdBinding), initplan.BindingSelectionInput{
				Binding: createdBinding.ID,
			})
			if err != nil {
				return err
			}
		}
	}

	printInitPlanReview(runtime, plan)
	return validatePreviewAndMaybeWriteLazyopsYAML(runtime, plan, initArgs)
}

func parseInitArgs(args []string) (initArgs, error) {
	flagSet := flag.NewFlagSet("init", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	project := flagSet.String("project", "", "project id or slug")
	runtimeMode := flagSet.String("runtime-mode", "", "runtime mode: standalone, distributed-mesh, distributed-k3s")
	target := flagSet.String("target", "", "target id or name")
	binding := flagSet.String("binding", "", "existing binding id, name, or target_ref")
	createBinding := flagSet.Bool("create-binding", false, "create a new deployment binding")
	bindingName := flagSet.String("binding-name", "", "name for a newly created deployment binding")
	writeFile := flagSet.Bool("write", false, "write lazyops.yaml to the repository root after review")
	overwrite := flagSet.Bool("overwrite", false, "confirm overwrite if lazyops.yaml already exists")
	magicDomainProvider := flagSet.String("magic-domain-provider", "", "magic domain provider: sslip.io or nip.io")
	scaleToZero := flagSet.Bool("scale-to-zero", false, "opt in the generated lazyops.yaml scale_to_zero_policy")

	if err := flagSet.Parse(args); err != nil {
		return initArgs{}, errors.New("invalid init flags. next: use `lazyops init [--project <project-id-or-slug>] [--runtime-mode <mode>] [--target <id|name>] [--binding <binding-id|name|target-ref> | --create-binding [--binding-name <name>]] [--magic-domain-provider <sslip.io|nip.io>] [--scale-to-zero] [--write [--overwrite]]`")
	}
	if flagSet.NArg() > 0 {
		return initArgs{}, fmt.Errorf("unexpected init arguments: %s. next: use flags instead of positional arguments", strings.Join(flagSet.Args(), " "))
	}

	mode, err := initplan.ParseRuntimeMode(*runtimeMode)
	if err != nil {
		return initArgs{}, err
	}
	if strings.TrimSpace(*binding) != "" && *createBinding {
		return initArgs{}, errors.New("choose one binding action only. next: use either `--binding <binding-id|name|target-ref>` or `--create-binding`")
	}
	if strings.TrimSpace(*bindingName) != "" && !*createBinding {
		return initArgs{}, errors.New("`--binding-name` requires `--create-binding`. next: rerun `lazyops init --create-binding --binding-name <name>`")
	}
	if *overwrite && !*writeFile {
		return initArgs{}, errors.New("`--overwrite` requires `--write`. next: rerun `lazyops init --write --overwrite`")
	}

	provider := strings.TrimSpace(*magicDomainProvider)
	if provider != "" && !isAllowedMagicDomainProvider(provider) {
		return initArgs{}, fmt.Errorf("magic domain provider %q is invalid. next: use `--magic-domain-provider sslip.io` or `--magic-domain-provider nip.io`", provider)
	}

	return initArgs{
		Project:             strings.TrimSpace(*project),
		RuntimeMode:         mode,
		Target:              strings.TrimSpace(*target),
		Binding:             strings.TrimSpace(*binding),
		CreateBinding:       *createBinding,
		BindingName:         strings.TrimSpace(*bindingName),
		Write:               *writeFile,
		Overwrite:           *overwrite,
		MagicDomainProvider: provider,
		ScaleToZero:         *scaleToZero,
	}, nil
}

type initDiscovery struct {
	projects     []contracts.Project
	instances    []contracts.Instance
	meshNetworks []contracts.MeshNetwork
	clusters     []contracts.Cluster
}

func fetchInitDiscovery(ctx context.Context, runtime *Runtime, credential credentials.Record) (initDiscovery, error) {
	projectsResponse, err := fetchProjects(ctx, runtime, credential)
	if err != nil {
		return initDiscovery{}, err
	}
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
		projects:     projectsResponse.Projects,
		instances:    instancesResponse.Instances,
		meshNetworks: meshNetworksResponse.MeshNetworks,
		clusters:     clustersResponse.Clusters,
	}, nil
}

func fetchProjects(ctx context.Context, runtime *Runtime, credential credentials.Record) (contracts.ProjectsResponse, error) {
	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "GET",
		Path:   "/api/v1/projects",
	}, credential))
	if err != nil {
		return contracts.ProjectsResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.ProjectsResponse{}, parseAPIError(response)
	}
	return contracts.DecodeProjectsResponse(response.Body)
}

func fetchInstances(ctx context.Context, runtime *Runtime, credential credentials.Record) (contracts.InstancesResponse, error) {
	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "GET",
		Path:   "/api/v1/instances",
	}, credential))
	if err != nil {
		return contracts.InstancesResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.InstancesResponse{}, parseAPIError(response)
	}
	return contracts.DecodeInstancesResponse(response.Body)
}

func fetchMeshNetworks(ctx context.Context, runtime *Runtime, credential credentials.Record) (contracts.MeshNetworksResponse, error) {
	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "GET",
		Path:   "/api/v1/mesh-networks",
	}, credential))
	if err != nil {
		return contracts.MeshNetworksResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.MeshNetworksResponse{}, parseAPIError(response)
	}
	return contracts.DecodeMeshNetworksResponse(response.Body)
}

func fetchClusters(ctx context.Context, runtime *Runtime, credential credentials.Record) (contracts.ClustersResponse, error) {
	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "GET",
		Path:   "/api/v1/clusters",
	}, credential))
	if err != nil {
		return contracts.ClustersResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.ClustersResponse{}, parseAPIError(response)
	}
	return contracts.DecodeClustersResponse(response.Body)
}

func fetchBindings(ctx context.Context, runtime *Runtime, credential credentials.Record, projectID string) (contracts.DeploymentBindingsResponse, error) {
	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "GET",
		Path:   fmt.Sprintf("/api/v1/projects/%s/deployment-bindings", projectID),
	}, credential))
	if err != nil {
		return contracts.DeploymentBindingsResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.DeploymentBindingsResponse{}, parseAPIError(response)
	}
	return contracts.DecodeDeploymentBindingsResponse(response.Body)
}

type createDeploymentBindingRequest struct {
	Name        string `json:"name"`
	TargetRef   string `json:"target_ref"`
	RuntimeMode string `json:"runtime_mode"`
	TargetKind  string `json:"target_kind"`
	TargetID    string `json:"target_id"`
}

func createDeploymentBinding(ctx context.Context, runtime *Runtime, credential credentials.Record, projectID string, binding initplan.BindingSummary) (contracts.DeploymentBinding, error) {
	body := mustMarshalJSON(createDeploymentBindingRequest{
		Name:        binding.Name,
		TargetRef:   binding.TargetRef,
		RuntimeMode: string(binding.RuntimeMode),
		TargetKind:  binding.TargetKind,
		TargetID:    binding.TargetID,
	})

	response, err := runtime.Transport.Do(ctx, authorizeRequest(transport.Request{
		Method: "POST",
		Path:   fmt.Sprintf("/api/v1/projects/%s/deployment-bindings", projectID),
		Body:   body,
	}, credential))
	if err != nil {
		return contracts.DeploymentBinding{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contracts.DeploymentBinding{}, parseAPIError(response)
	}
	return contracts.DecodeDeploymentBinding(response.Body)
}

func mustMarshalJSON(value any) []byte {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return body
}

func printInitPlanReview(runtime *Runtime, plan initplan.InitPlan) {
	runtime.Output.Success("init plan review ready")
	runtime.Output.Info("transport mode: %s", runtime.Transport.Mode())
	runtime.Output.Info("repo root: %s", plan.RepoRoot)
	runtime.Output.Info("repo layout: %s", plan.Layout)
	runtime.Output.Info("compatibility policy: env_injection=%t managed_credentials=%t localhost_rescue=%t", plan.CompatibilityPolicy.EnvInjection, plan.CompatibilityPolicy.ManagedCredentials, plan.CompatibilityPolicy.LocalhostRescue)
	runtime.Output.Info("target summaries are sanitized; raw IPs and secret refs stay outside the repo contract")

	if plan.SelectedProject != nil {
		runtime.Output.Info("selected project: %s (%s)", plan.SelectedProject.Name, plan.SelectedProject.Slug)
	} else {
		runtime.Output.Warn("project selection pending; rerun `lazyops init --project <project-id-or-slug>` if multiple projects are listed")
		for _, project := range plan.Projects {
			runtime.Output.Info("project option: %s (%s)", project.Name, project.Slug)
		}
	}

	if plan.RuntimeMode != "" {
		runtime.Output.Info("selected runtime mode: %s", plan.RuntimeMode)
		if plan.RuntimeMode == initplan.RuntimeModeDistributedK3s {
			runtime.Output.Info("distributed-k3s boundary: K3s remains the workload scheduler; CLI writes logical binding refs only")
		}
	} else {
		runtime.Output.Warn("runtime mode selection pending; use `--runtime-mode <standalone|distributed-mesh|distributed-k3s>`")
		for _, mode := range []initplan.RuntimeMode{initplan.RuntimeModeStandalone, initplan.RuntimeModeDistributedMesh, initplan.RuntimeModeDistributedK3s} {
			runtime.Output.Info("runtime mode option: %s", mode)
		}
	}

	if len(plan.Services) == 0 {
		runtime.Output.Warn("no service markers found yet; expected one of package.json, go.mod, requirements.txt, or Dockerfile")
	} else {
		for _, service := range plan.Services {
			runtime.Output.Info("service %s -> %s (%s)", service.Name, service.Path, strings.Join(service.Signals, ", "))
			if strings.TrimSpace(service.StartHint) != "" {
				runtime.Output.Info("start hint for %s: %s", service.Name, service.StartHint)
			}
			if strings.TrimSpace(service.Healthcheck.Path) != "" {
				if service.Healthcheck.Port > 0 {
					runtime.Output.Info("health hint for %s: %s on %d", service.Name, service.Healthcheck.Path, service.Healthcheck.Port)
				} else {
					runtime.Output.Info("health hint for %s: %s", service.Name, service.Healthcheck.Path)
				}
			}
			for _, warning := range service.Warnings {
				runtime.Output.Warn("service %s: %s", service.Name, warning)
			}
		}
	}

	printTargetReview(runtime, plan)
	printBindingReview(runtime, plan)

	if len(plan.DependencyBindings) == 0 {
		if plan.RuntimeMode == initplan.RuntimeModeDistributedMesh && len(plan.Services) > 1 {
			runtime.Output.Warn("distributed-mesh review found multiple services but no dependency bindings were inferred yet")
		}
		runtime.Output.Info("dependency bindings: none inferred yet")
		return
	}

	for _, binding := range plan.DependencyBindings {
		runtime.Output.Info("dependency binding %s.%s -> %s (%s)", binding.Service, binding.Alias, binding.TargetService, binding.Protocol)
	}
}

func printTargetReview(runtime *Runtime, plan initplan.InitPlan) {
	if len(plan.Targets) == 0 {
		runtime.Output.Warn("no deployment targets are currently available")
		return
	}

	if plan.RuntimeMode != "" {
		targets := plan.TargetsForMode(plan.RuntimeMode)
		for _, target := range targets {
			runtime.Output.Info("target option for %s: %s %s [%s]", plan.RuntimeMode, target.Kind, target.Name, target.Status)
		}
		if plan.SelectedTarget != nil {
			runtime.Output.Info("selected target: %s %s [%s]", plan.SelectedTarget.Kind, plan.SelectedTarget.Name, plan.SelectedTarget.Status)
		} else {
			runtime.Output.Warn("target selection pending; use `--target <id|name>` after choosing a runtime mode")
		}
		return
	}

	for _, mode := range []initplan.RuntimeMode{initplan.RuntimeModeStandalone, initplan.RuntimeModeDistributedMesh, initplan.RuntimeModeDistributedK3s} {
		targets := plan.TargetsForMode(mode)
		if len(targets) == 0 {
			runtime.Output.Warn("no targets available for runtime mode %s", mode)
			continue
		}
		for _, target := range targets {
			runtime.Output.Info("target option for %s: %s %s [%s]", mode, target.Kind, target.Name, target.Status)
		}
	}
}

func printBindingReview(runtime *Runtime, plan initplan.InitPlan) {
	if plan.SelectedProject == nil {
		runtime.Output.Warn("binding selection is waiting for a project choice")
		return
	}

	filtered := plan.Bindings
	if plan.RuntimeMode != "" {
		filtered = initplan.FilterBindingsByRuntimeMode(filtered, plan.RuntimeMode)
	}
	if plan.SelectedTarget != nil {
		filtered = initplan.FilterBindingsByTarget(filtered, *plan.SelectedTarget)
	}

	if len(filtered) == 0 {
		runtime.Output.Warn("no existing deployment binding matches the current project/target selection")
		if plan.RuntimeMode != "" && plan.SelectedTarget != nil {
			runtime.Output.Warn("binding selection pending; use `--create-binding [--binding-name <name>]` to create one or adjust project/runtime/target selection")
		}
	} else {
		for _, binding := range filtered {
			runtime.Output.Info("binding option: %s -> %s (%s, %s)", binding.Name, binding.TargetRef, binding.RuntimeMode, binding.TargetKind)
		}
	}

	if plan.SelectedBinding != nil {
		runtime.Output.Info("selected binding: %s -> %s (%s, %s)", plan.SelectedBinding.Name, plan.SelectedBinding.TargetRef, plan.SelectedBinding.RuntimeMode, plan.SelectedBinding.TargetKind)
		return
	}

	if len(filtered) > 1 {
		runtime.Output.Warn("binding selection pending; use `--binding <binding-id|name|target-ref>` to reuse one of the listed bindings")
	}
}

func validatePreviewAndMaybeWriteLazyopsYAML(runtime *Runtime, plan initplan.InitPlan, initArgs initArgs) error {
	missing := missingLazyopsYAMLRequirements(plan)
	if len(missing) > 0 {
		runtime.Output.Warn("lazyops.yaml generation pending; complete %s before writing", strings.Join(missing, ", "))
		if initArgs.Write {
			return fmt.Errorf("cannot write lazyops.yaml yet. next: select %s before rerunning `lazyops init --write`", strings.Join(missing, ", "))
		}
		return nil
	}

	payload, err := lazyyaml.Generate(plan, generateOptionsFromInitArgs(initArgs))
	if err != nil {
		return fmt.Errorf("lazyops.yaml local validation failed. next: fix the init selections or detected services and retry: %w", err)
	}

	configPath := lazyyaml.DefaultPath(plan.RepoRoot)
	runtime.Output.Success("lazyops.yaml local validation passed")
	runtime.Output.Info("lazyops.yaml path: %s", configPath)
	runtime.Output.Info("pre-write review:")
	runtime.Output.Print("")
	runtime.Output.Print("%s", string(payload))

	if !initArgs.Write {
		runtime.Output.Warn("write pending; rerun `lazyops init --write` to create lazyops.yaml at the repository root")
		return nil
	}

	result, err := lazyyaml.WriteFile(plan.RepoRoot, payload, initArgs.Overwrite)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("could not write lazyops.yaml due to filesystem permissions. next: fix write access to the repo root and retry: %w", err)
		}
		return err
	}

	if result.Overwrote {
		runtime.Output.Success("lazyops.yaml written after confirmed overwrite")
		runtime.Output.Info("backup created: %s", result.BackupPath)
	} else {
		runtime.Output.Success("lazyops.yaml written")
	}
	runtime.Output.Info("written path: %s", result.Path)
	runtime.Output.Success("init complete for %s", plan.RuntimeMode)
	return nil
}

func missingLazyopsYAMLRequirements(plan initplan.InitPlan) []string {
	missing := make([]string, 0, 3)
	if plan.SelectedProject == nil {
		missing = append(missing, "project selection")
	}
	if plan.RuntimeMode == "" {
		missing = append(missing, "runtime mode selection")
	}
	if plan.SelectedBinding == nil {
		missing = append(missing, "deployment binding selection")
	}
	return missing
}

func generateOptionsFromInitArgs(initArgs initArgs) lazyyaml.GenerateOptions {
	options := lazyyaml.GenerateOptions{}
	if strings.TrimSpace(initArgs.MagicDomainProvider) != "" {
		options.MagicDomainProvider = initArgs.MagicDomainProvider
	}
	if initArgs.ScaleToZero {
		options.ScaleToZeroEnabled = boolPointer(true)
	}
	return options
}

func isAllowedMagicDomainProvider(provider string) bool {
	normalized := strings.ToLower(strings.TrimSpace(provider))
	for _, allowed := range lazyyaml.AllowedMagicDomainProviders() {
		if normalized == allowed {
			return true
		}
	}
	return false
}

func boolPointer(value bool) *bool {
	return &value
}
