package runtime

import (
	"context"
	"errors"
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
