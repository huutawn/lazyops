package reporting

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/state"
)

func TestEvaluateHealthMapsLifecycleToStatus(t *testing.T) {
	reporter := New(slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	now := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
	reporter.now = func() time.Time { return now }

	cases := []struct {
		name        string
		current     contracts.AgentState
		sessionID   string
		fingerprint string
		want        contracts.AgentHealthStatus
	}{
		{name: "online", current: contracts.AgentStateConnected, sessionID: "sess_1", fingerprint: "cap_1", want: contracts.AgentHealthOnline},
		{name: "degraded", current: contracts.AgentStateDegraded, sessionID: "sess_1", fingerprint: "cap_1", want: contracts.AgentHealthDegraded},
		{name: "offline", current: contracts.AgentStateDisconnected, sessionID: "sess_1", fingerprint: "cap_1", want: contracts.AgentHealthOffline},
		{name: "busy reconcile", current: contracts.AgentStateReconciling, sessionID: "sess_1", fingerprint: "cap_1", want: contracts.AgentHealthBusy},
		{name: "busy reporting", current: contracts.AgentStateReporting, sessionID: "sess_1", fingerprint: "cap_1", want: contracts.AgentHealthBusy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			local := &state.AgentLocalState{
				Metadata: state.AgentMetadata{
					AgentID:      "agt_local",
					RuntimeMode:  contracts.RuntimeModeStandalone,
					AgentKind:    contracts.AgentKindInstance,
					CurrentState: tc.current,
				},
				Enrollment: state.EnrollmentState{
					SessionID: tc.sessionID,
				},
				CapabilitySnapshot: state.CapabilitySnapshotState{
					Fingerprint: tc.fingerprint,
				},
			}

			health := reporter.EvaluateHealth(local)
			if health.Status != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, health.Status)
			}
			if health.UpdatedAt != now {
				t.Fatalf("expected updated_at %v, got %v", now, health.UpdatedAt)
			}
		})
	}
}

func TestBuildHeartbeatUsesCapabilityUpdateModes(t *testing.T) {
	reporter := New(slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	now := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
	reporter.now = func() time.Time { return now }

	local := &state.AgentLocalState{
		Metadata: state.AgentMetadata{
			AgentID:       "agt_local",
			RuntimeMode:   contracts.RuntimeModeStandalone,
			AgentKind:     contracts.AgentKindInstance,
			CurrentState:  contracts.AgentStateConnected,
			LastStartedAt: now.Add(-5 * time.Minute),
		},
		Enrollment: state.EnrollmentState{
			SessionID: "sess_1",
		},
	}

	baseCapabilities := contracts.CapabilityReportPayload{
		AgentKind:   contracts.AgentKindInstance,
		RuntimeMode: contracts.RuntimeModeStandalone,
		ControlChannel: contracts.ControlChannelCapability{
			WebSocketPath: contracts.ControlWebSocketPath,
			OutboundOnly:  true,
			Reconnectable: true,
		},
		Network: contracts.NetworkCapability{
			OutboundOnly:           true,
			PrivateOverlay:         false,
			CrossNodePrivateOnly:   false,
			SupportedMeshProviders: []contracts.MeshProvider{contracts.MeshProviderWireGuard},
		},
		Gateway: contracts.GatewayCapability{
			Enabled:      true,
			Provider:     "caddy",
			MagicDomains: []string{"sslip.io", "nip.io"},
			HTTPSManaged: true,
		},
		Sidecar: contracts.SidecarCapability{
			Enabled:                   true,
			Precedence:                []string{"env_injection", "managed_credentials", "localhost_rescue"},
			SupportsHTTP:              true,
			SupportsTCP:               true,
			SupportsLocalhostRescue:   true,
			SupportsManagedCredential: true,
		},
		Telemetry: contracts.TelemetryCapability{
			LogCollection:     true,
			MetricRollup:      true,
			TraceSummary:      true,
			TopologyReporting: true,
			IncidentReporting: true,
		},
		Node: contracts.NodeCapability{
			NodeMetrics: true,
		},
	}

	if err := reporter.ReconcileCapabilitySnapshot(&local.CapabilitySnapshot, baseCapabilities); err != nil {
		t.Fatalf("reconcile base capability snapshot: %v", err)
	}

	fullHeartbeat, health, err := reporter.BuildHeartbeat(local)
	if err != nil {
		t.Fatalf("build full heartbeat: %v", err)
	}
	if health.Status != contracts.AgentHealthOnline {
		t.Fatalf("expected online health, got %q", health.Status)
	}
	if fullHeartbeat.CapabilityUpdate == nil || fullHeartbeat.CapabilityUpdate.Mode != contracts.CapabilityUpdateFull {
		t.Fatalf("expected full capability update, got %#v", fullHeartbeat.CapabilityUpdate)
	}
	if fullHeartbeat.Capabilities.AgentKind != contracts.AgentKindInstance {
		t.Fatal("expected full heartbeat to embed full capabilities")
	}

	reporter.MarkCapabilitiesReported(&local.CapabilitySnapshot)

	unchangedHeartbeat, _, err := reporter.BuildHeartbeat(local)
	if err != nil {
		t.Fatalf("build unchanged heartbeat: %v", err)
	}
	if unchangedHeartbeat.CapabilityUpdate == nil || unchangedHeartbeat.CapabilityUpdate.Mode != contracts.CapabilityUpdateUnchanged {
		t.Fatalf("expected unchanged capability update, got %#v", unchangedHeartbeat.CapabilityUpdate)
	}

	updatedCapabilities := baseCapabilities
	updatedCapabilities.AdditionalCapabilities = map[string]bool{"localhost_rescue_cross_node": true}
	if err := reporter.ReconcileCapabilitySnapshot(&local.CapabilitySnapshot, updatedCapabilities); err != nil {
		t.Fatalf("reconcile updated capability snapshot: %v", err)
	}

	diffHeartbeat, _, err := reporter.BuildHeartbeat(local)
	if err != nil {
		t.Fatalf("build diff heartbeat: %v", err)
	}
	if diffHeartbeat.CapabilityUpdate == nil || diffHeartbeat.CapabilityUpdate.Mode != contracts.CapabilityUpdateDiff {
		t.Fatalf("expected diff capability update, got %#v", diffHeartbeat.CapabilityUpdate)
	}
	if diffHeartbeat.CapabilityUpdate.Version <= diffHeartbeat.CapabilityUpdate.BaseVersion {
		t.Fatal("expected capability version to advance for a diff update")
	}

	raw, ok := diffHeartbeat.CapabilityUpdate.Changed["additional_capabilities"]
	if !ok {
		t.Fatalf("expected additional_capabilities to be present in changed fields: %#v", diffHeartbeat.CapabilityUpdate.ChangedFields)
	}

	var changed map[string]bool
	if err := json.Unmarshal(raw, &changed); err != nil {
		t.Fatalf("unmarshal changed capability payload: %v", err)
	}
	if !changed["localhost_rescue_cross_node"] {
		t.Fatalf("expected diff payload to include the new capability flag, got %#v", changed)
	}
}
