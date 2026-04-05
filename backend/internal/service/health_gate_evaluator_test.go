package service

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"lazyops-server/internal/models"
)

func serverPort(s *httptest.Server) int {
	_, port, _ := net.SplitHostPort(s.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	return p
}

func TestProductionHealthGateEvaluatorSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	port := serverPort(server)
	ip := "127.0.0.1"
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_123",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  &ip,
		Status:     "online",
		LabelsJSON: `{"services":["api"]}`,
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		Name:       "api",
		TargetRef:  "api-main",
		TargetKind: "instance",
		TargetID:   "inst_123",
	})

	compiledJSON, _ := json.Marshal(map[string]any{
		"deployment_binding_id": "bind_123",
		"services": []map[string]any{
			{
				"name": "api",
				"path": "apps/api",
				"healthcheck": map[string]any{
					"protocol": "http",
					"port":     float64(port),
					"path":     "/health",
				},
			},
		},
	})

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		Status:               "planned",
		CompiledRevisionJSON: string(compiledJSON),
	})

	evaluator := NewProductionHealthGateEvaluator(instanceStore, revisionStore, bindingStore)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := evaluator.Evaluate(ctx, "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("evaluate health gate: %v", err)
	}

	if !result.Passed {
		t.Fatal("expected health gate to pass")
	}
	if len(result.Services) != 1 {
		t.Fatalf("expected 1 service result, got %d", len(result.Services))
	}
	if !result.Services[0].Healthy {
		t.Fatalf("expected service api to be healthy, got unhealthy: %s", result.Services[0].Message)
	}
}

func TestProductionHealthGateEvaluatorFailsWhenServiceUnhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	port := serverPort(server)
	ip := "127.0.0.1"
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_123",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  &ip,
		Status:     "online",
		LabelsJSON: `{"services":["api"]}`,
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		Name:       "api",
		TargetRef:  "api-main",
		TargetKind: "instance",
		TargetID:   "inst_123",
	})

	compiledJSON, _ := json.Marshal(map[string]any{
		"deployment_binding_id": "bind_123",
		"services": []map[string]any{
			{
				"name": "api",
				"path": "apps/api",
				"healthcheck": map[string]any{
					"protocol":          "http",
					"port":              float64(port),
					"path":              "/health",
					"success_threshold": 1.0,
					"failure_threshold": 1.0,
				},
			},
		},
	})

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		Status:               "planned",
		CompiledRevisionJSON: string(compiledJSON),
	})

	evaluator := NewProductionHealthGateEvaluator(instanceStore, revisionStore, bindingStore)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := evaluator.Evaluate(ctx, "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("evaluate health gate: %v", err)
	}

	if result.Passed {
		t.Fatal("expected health gate to fail when service returns 500")
	}
	if result.Services[0].Healthy {
		t.Fatal("expected service api to be unhealthy")
	}
}

func TestProductionHealthGateEvaluatorTCPProbe(t *testing.T) {
	ip := "127.0.0.1"
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_123",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  &ip,
		Status:     "online",
		LabelsJSON: `{"services":["db"]}`,
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		Name:       "db",
		TargetRef:  "db-main",
		TargetKind: "instance",
		TargetID:   "inst_123",
	})

	compiledJSON, _ := json.Marshal(map[string]any{
		"deployment_binding_id": "bind_123",
		"services": []map[string]any{
			{
				"name": "db",
				"path": "apps/db",
				"healthcheck": map[string]any{
					"protocol": "tcp",
					"port":     5432,
				},
			},
		},
	})

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		Status:               "planned",
		CompiledRevisionJSON: string(compiledJSON),
	})

	evaluator := NewProductionHealthGateEvaluator(instanceStore, revisionStore, bindingStore)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := evaluator.Evaluate(ctx, "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("evaluate health gate: %v", err)
	}

	if result.Passed {
		t.Fatal("expected health gate to fail when TCP port is not listening")
	}
}

func TestProductionHealthGateEvaluatorRejectsOfflineInstance(t *testing.T) {
	ip := "127.0.0.1"
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_offline",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  &ip,
		Status:     "offline",
		LabelsJSON: `{"services":["api"]}`,
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		Name:       "api",
		TargetRef:  "api-main",
		TargetKind: "instance",
		TargetID:   "inst_offline",
	})

	compiledJSON, _ := json.Marshal(map[string]any{
		"deployment_binding_id": "bind_123",
		"services":              []map[string]any{},
	})

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		Status:               "planned",
		CompiledRevisionJSON: string(compiledJSON),
	})

	evaluator := NewProductionHealthGateEvaluator(instanceStore, revisionStore, bindingStore)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := evaluator.Evaluate(ctx, "prj_123", "dep_123", "rev_123")
	if err == nil {
		t.Fatal("expected error for offline instance")
	}
}

func TestProductionHealthGateEvaluatorRetrySucceeds(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	port := serverPort(server)
	ip := "127.0.0.1"
	instanceStore := newFakeInstanceStore(&models.Instance{
		ID:         "inst_123",
		UserID:     "usr_123",
		Name:       "edge-sg-1",
		PublicIP:   ptrString("203.0.113.10"),
		PrivateIP:  &ip,
		Status:     "online",
		LabelsJSON: `{"services":["api"]}`,
	})

	bindingStore := newFakeDeploymentBindingStore(&models.DeploymentBinding{
		ID:         "bind_123",
		ProjectID:  "prj_123",
		Name:       "api",
		TargetRef:  "api-main",
		TargetKind: "instance",
		TargetID:   "inst_123",
	})

	compiledJSON, _ := json.Marshal(map[string]any{
		"deployment_binding_id": "bind_123",
		"services": []map[string]any{
			{
				"name": "api",
				"path": "apps/api",
				"healthcheck": map[string]any{
					"protocol":          "http",
					"port":              float64(port),
					"path":              "/health",
					"success_threshold": 1.0,
					"failure_threshold": 5.0,
				},
			},
		},
	})

	revisionStore := newFakeDesiredStateRevisionStore(&models.DesiredStateRevision{
		ID:                   "rev_123",
		ProjectID:            "prj_123",
		BlueprintID:          "bp_123",
		DeploymentBindingID:  "bind_123",
		CommitSHA:            "abc123",
		Status:               "planned",
		CompiledRevisionJSON: string(compiledJSON),
	})

	evaluator := NewProductionHealthGateEvaluator(instanceStore, revisionStore, bindingStore)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := evaluator.Evaluate(ctx, "prj_123", "dep_123", "rev_123")
	if err != nil {
		t.Fatalf("evaluate health gate: %v", err)
	}

	if !result.Passed {
		t.Fatalf("expected health gate to pass after retry, got: %s", result.Services[0].Message)
	}
}
