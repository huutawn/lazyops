package runtime

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
)

func TestBuildPortApplyObservationDetectsMismatch(t *testing.T) {
	spec := contracts.K3sServiceSpecPayload{
		Name:        "api",
		TargetPort:  8080,
		ServicePort: 80,
	}
	deploymentJSON := []byte(`{
		"spec":{"template":{"spec":{"containers":[{"name":"api","ports":[{"containerPort":3000,"name":"http"}]}]}}}
	}`)
	serviceJSON := []byte(`{
		"spec":{"ports":[{"port":80,"targetPort":"http"}]}
	}`)

	observation := buildPortApplyObservation(spec, deploymentJSON, serviceJSON)
	if observation.Status != "mismatch" {
		t.Fatalf("expected mismatch status, got %#v", observation)
	}
	if observation.ObservedServiceTargetPort != 3000 {
		t.Fatalf("expected resolved named target port 3000, got %#v", observation)
	}
}

func TestBuildPortApplyObservationMatchesExpectedPorts(t *testing.T) {
	spec := contracts.K3sServiceSpecPayload{
		Name:        "web",
		TargetPort:  3000,
		ServicePort: 80,
	}
	deploymentJSON := []byte(`{
		"spec":{"template":{"spec":{"containers":[{"name":"web","ports":[{"containerPort":3000,"name":"http"}]}]}}}
	}`)
	serviceJSON := []byte(`{
		"spec":{"ports":[{"port":80,"targetPort":"http"}]}
	}`)

	observation := buildPortApplyObservation(spec, deploymentJSON, serviceJSON)
	if observation.Status != "matched" {
		t.Fatalf("expected matched status, got %#v", observation)
	}
	if observation.Warning != "" {
		t.Fatalf("expected no warning, got %#v", observation)
	}
}

func TestBuildRolloutProgressReady(t *testing.T) {
	progress := buildRolloutProgress("api", []byte(`{
		"metadata":{"generation":3},
		"status":{
			"observedGeneration":3,
			"replicas":2,
			"readyReplicas":2,
			"updatedReplicas":2,
			"availableReplicas":2,
			"conditions":[{"type":"Available","status":"True","message":"all replicas ready"}]
		}
	}`), time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC))

	if progress.Status != "ready" {
		t.Fatalf("expected ready status, got %#v", progress)
	}
	if progress.ReadyReplicas != 2 || progress.DesiredReplicas != 2 {
		t.Fatalf("unexpected rollout progress: %#v", progress)
	}
}

func TestBuildIngressObservationUsesGatewayAddressFallback(t *testing.T) {
	observation := buildIngressObservation("api", contracts.PublicDomainPayload{
		ServiceName:  "api",
		FallbackHost: "api.demo.127.0.0.1.nip.io",
		FallbackURL:  "http://api.demo.127.0.0.1.nip.io",
	}, []byte(`{
		"metadata":{"name":"api"},
		"spec":{"rules":[{"host":"api.demo.127.0.0.1.nip.io"}]},
		"status":{"loadBalancer":{"ingress":[]}}
	}`), []string{"10.10.10.10"}, time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC))

	if !observation.Ready {
		t.Fatalf("expected ingress to be ready with gateway address fallback, got %#v", observation)
	}
	if len(observation.ExternalAddresses) != 1 || observation.ExternalAddresses[0] != "10.10.10.10" {
		t.Fatalf("unexpected external addresses: %#v", observation)
	}
}

func TestParseK3sTimestampedLogLine(t *testing.T) {
	timestamp, message := parseK3sTimestampedLogLine("2026-04-16T10:00:00Z server started")
	if timestamp.IsZero() {
		t.Fatal("expected timestamp to be parsed")
	}
	if message != "server started" {
		t.Fatalf("expected message without timestamp, got %q", message)
	}
}

func TestSummarizeKubectlApplyOutputDetectsIdempotentApply(t *testing.T) {
	summary := summarizeKubectlApplyOutput([]byte("deployment.apps/api unchanged\nservice/api unchanged\n"))
	if summary.Unchanged != 2 {
		t.Fatalf("expected 2 unchanged resources, got %#v", summary)
	}
	if summary.totalMutations() != 0 {
		t.Fatalf("expected zero mutations, got %#v", summary)
	}
}

func TestClassifyKubectlErrorDetectsStaleKubeconfig(t *testing.T) {
	err := classifyKubectlError(context.Background(), []string{"get", "pods"}, []byte("Unauthorized"), errors.New("exit status 1"))
	var opErr *OperationError
	if !errors.As(err, &opErr) {
		t.Fatalf("expected OperationError, got %T", err)
	}
	if opErr.Code != "k8s_kubeconfig_stale" {
		t.Fatalf("expected stale kubeconfig code, got %#v", opErr)
	}
}

func TestMaterializeApplyManifestsSplitsNamespaceFromResources(t *testing.T) {
	driver := NewK3sDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "kubectl", "")
	runtimeCtx := RuntimeContext{
		Project: ProjectMetadata{Namespace: "lazyops-test"},
		Revision: contracts.DesiredRevisionPayload{
			ManifestBundle: contracts.ManifestBundlePayload{
				Documents: []contracts.ManifestDocumentPayload{
					{
						Kind:    "Namespace",
						Name:    "lazyops-test",
						Content: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: lazyops-test\n",
					},
					{
						Kind:    "Service",
						Name:    "api",
						Content: "apiVersion: v1\nkind: Service\nmetadata:\n  name: api\n  namespace: lazyops-test\n",
					},
					{
						Kind:    "Deployment",
						Name:    "api",
						Content: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n  namespace: lazyops-test\n",
					},
				},
			},
		},
	}

	plan, err := driver.materializeApplyManifests(t.TempDir(), runtimeCtx)
	if err != nil {
		t.Fatalf("materialize apply manifests: %v", err)
	}
	if plan.NamespacePath == "" || plan.ResourcesPath == "" {
		t.Fatalf("expected both namespace and resource manifests, got %#v", plan)
	}

	namespaceRaw, err := os.ReadFile(plan.NamespacePath)
	if err != nil {
		t.Fatalf("read namespace manifest: %v", err)
	}
	if !strings.Contains(string(namespaceRaw), "kind: Namespace") {
		t.Fatalf("expected namespace manifest to contain namespace doc, got %q", string(namespaceRaw))
	}
	if strings.Contains(string(namespaceRaw), "kind: Service") {
		t.Fatalf("expected namespace manifest to exclude service docs, got %q", string(namespaceRaw))
	}

	resourceRaw, err := os.ReadFile(plan.ResourcesPath)
	if err != nil {
		t.Fatalf("read resources manifest: %v", err)
	}
	if strings.Contains(string(resourceRaw), "kind: Namespace") {
		t.Fatalf("expected resource manifest to exclude namespace docs, got %q", string(resourceRaw))
	}
	if !strings.Contains(string(resourceRaw), "kind: Service") || !strings.Contains(string(resourceRaw), "kind: Deployment") {
		t.Fatalf("expected resource manifest to contain service and deployment docs, got %q", string(resourceRaw))
	}
}

func TestReconcileRevisionAppliesNamespaceBeforeDryRun(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "kubectl.log")
	kubectlPath := filepath.Join(root, "kubectl")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"" + logPath + "\"\n" +
		"case \"$*\" in\n" +
		"  *\"get namespace lazyops-test -o name\"*)\n" +
		"    printf 'namespace/lazyops-test\\n'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  *\"rollout status deployment/api\"*)\n" +
		"    printf 'deployment \"api\" successfully rolled out\\n'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  *\"get deployment/api -o json\"*)\n" +
		"    printf '{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"api\",\"ports\":[{\"containerPort\":8080,\"name\":\"http\"}]}]}}},\"status\":{\"observedGeneration\":1,\"replicas\":1,\"readyReplicas\":1,\"updatedReplicas\":1,\"availableReplicas\":1}}'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  *\"get service/api -o json\"*)\n" +
		"    printf '{\"spec\":{\"ports\":[{\"port\":8080,\"targetPort\":\"http\"}]}}'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  *\"get service/traefik -o json\"*)\n" +
		"    printf '{}'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  *\"apply -f \"*\"k8s-manifest-namespaces.yaml\"*)\n" +
		"    printf 'namespace/lazyops-test created\\n'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  *\"apply --dry-run=server -f \"*\"k8s-manifest-resources.yaml\"*)\n" +
		"    printf 'service/api configured\\n'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"  *\"apply -f \"*\"k8s-manifest-resources.yaml\"*)\n" +
		"    printf 'service/api configured\\n'\n" +
		"    exit 0\n" +
		"    ;;\n" +
		"esac\n" +
		"printf 'unexpected kubectl args: %s\\n' \"$*\" >&2\n" +
		"exit 1\n"
	if err := os.WriteFile(kubectlPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake kubectl: %v", err)
	}

	driver := NewK3sDriver(slog.New(slog.NewTextHandler(io.Discard, nil)), root, kubectlPath, "")
	fixedNow := time.Date(2026, 4, 19, 5, 5, 0, 0, time.UTC)
	driver.now = func() time.Time { return fixedNow }

	payload := samplePreparePayload(contracts.RuntimeModeDistributedK3s)
	payload.Project.Namespace = "lazyops-test"
	payload.Revision.Namespace = "lazyops-test"
	payload.Revision.ServiceSpecs = []contracts.K3sServiceSpecPayload{
		{
			Name:        "api",
			Kind:        "app",
			Path:        "apps/api",
			TargetPort:  8080,
			ServicePort: 8080,
			Replicas:    1,
		},
	}
	payload.Revision.Services = nil
	payload.Revision.InternalBindings = nil
	payload.Revision.PublicDomains = nil
	payload.Revision.ManifestBundle = contracts.ManifestBundlePayload{
		Namespace: "lazyops-test",
		CombinedYAML: joinManifestDocuments([]string{
			"apiVersion: v1\nkind: Namespace\nmetadata:\n  name: lazyops-test",
			"apiVersion: v1\nkind: Service\nmetadata:\n  name: api\n  namespace: lazyops-test\nspec:\n  selector:\n    app.kubernetes.io/name: api\n  ports:\n    - port: 8080\n      targetPort: 8080",
			"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n  namespace: lazyops-test\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app.kubernetes.io/name: api\n  template:\n    metadata:\n      labels:\n        app.kubernetes.io/name: api\n    spec:\n      containers:\n        - name: api\n          image: nginxinc/nginx-unprivileged:stable-alpine\n          ports:\n            - containerPort: 8080\n              name: http",
		}),
		Documents: []contracts.ManifestDocumentPayload{
			{
				Kind:    "Namespace",
				Name:    "lazyops-test",
				Path:    "namespace.yaml",
				Content: "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: lazyops-test",
			},
			{
				Kind:    "Service",
				Name:    "api",
				Path:    "api-service.yaml",
				Content: "apiVersion: v1\nkind: Service\nmetadata:\n  name: api\n  namespace: lazyops-test\nspec:\n  selector:\n    app.kubernetes.io/name: api\n  ports:\n    - port: 8080\n      targetPort: 8080",
			},
			{
				Kind:    "Deployment",
				Name:    "api",
				Path:    "api-deployment.yaml",
				Content: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n  namespace: lazyops-test\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app.kubernetes.io/name: api\n  template:\n    metadata:\n      labels:\n        app.kubernetes.io/name: api\n    spec:\n      containers:\n        - name: api\n          image: nginxinc/nginx-unprivileged:stable-alpine\n          ports:\n            - containerPort: 8080\n              name: http",
			},
		},
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		t.Fatalf("build runtime context: %v", err)
	}
	if _, err := driver.ReconcileRevision(context.Background(), runtimeCtx); err != nil {
		t.Fatalf("reconcile revision: %v", err)
	}

	logRaw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read kubectl log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logRaw)), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected multiple kubectl calls, got %q", string(logRaw))
	}
	if !strings.Contains(lines[0], "apply -f") || !strings.Contains(lines[0], "k8s-manifest-namespaces.yaml") {
		t.Fatalf("expected namespace apply first, got %q", lines[0])
	}
	if strings.Contains(lines[0], "--dry-run=server") {
		t.Fatalf("expected namespace apply to skip server dry-run, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "get namespace lazyops-test -o name") {
		t.Fatalf("expected namespace readiness check second, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "apply --dry-run=server -f") || !strings.Contains(lines[2], "k8s-manifest-resources.yaml") {
		t.Fatalf("expected resource dry-run after namespace creation, got %q", lines[2])
	}
	if !strings.Contains(lines[3], "apply -f") || !strings.Contains(lines[3], "k8s-manifest-resources.yaml") {
		t.Fatalf("expected resource apply after dry-run, got %q", lines[3])
	}
}
