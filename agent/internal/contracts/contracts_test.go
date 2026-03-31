package contracts

import "testing"

func TestMinimumCommandSetHasHandlerBinding(t *testing.T) {
	if len(MinimumCommandSet) == 0 {
		t.Fatal("minimum command set must not be empty")
	}

	for _, command := range MinimumCommandSet {
		spec, ok := CommandHandlerBindings[command]
		if !ok {
			t.Fatalf("missing handler binding for command %q", command)
		}
		if spec.Command != command {
			t.Fatalf("binding command mismatch for %q", command)
		}
		if spec.Module == "" || spec.HandlerKey == "" {
			t.Fatalf("binding for command %q must include module and handler key", command)
		}
	}
}

func TestCommandHandlerBindingsDoNotDrift(t *testing.T) {
	if len(CommandHandlerBindings) != len(MinimumCommandSet) {
		t.Fatalf("expected %d command bindings, got %d", len(MinimumCommandSet), len(CommandHandlerBindings))
	}
}

func TestControlWebSocketPathIsLocked(t *testing.T) {
	if ControlWebSocketPath != "/ws/agents/control" {
		t.Fatalf("unexpected control websocket path: %s", ControlWebSocketPath)
	}
	if AckEnvelopeType != "command.ack" {
		t.Fatalf("unexpected ack envelope type: %s", AckEnvelopeType)
	}
	if NackEnvelopeType != "command.nack" {
		t.Fatalf("unexpected nack envelope type: %s", NackEnvelopeType)
	}
	if ErrorEnvelopeType != "command.error" {
		t.Fatalf("unexpected error envelope type: %s", ErrorEnvelopeType)
	}
}

func TestAgentHealthStatusesAreLocked(t *testing.T) {
	want := []struct {
		value AgentHealthStatus
		name  string
	}{
		{value: AgentHealthOnline, name: "online"},
		{value: AgentHealthDegraded, name: "degraded"},
		{value: AgentHealthOffline, name: "offline"},
		{value: AgentHealthBusy, name: "busy"},
	}

	for _, item := range want {
		if string(item.value) != item.name {
			t.Fatalf("expected health status %q, got %q", item.name, item.value)
		}
	}
}

func TestCapabilityUpdateModesAreLocked(t *testing.T) {
	want := []struct {
		value CapabilityUpdateMode
		name  string
	}{
		{value: CapabilityUpdateFull, name: "full"},
		{value: CapabilityUpdateDiff, name: "diff"},
		{value: CapabilityUpdateUnchanged, name: "unchanged"},
	}

	for _, item := range want {
		if string(item.value) != item.name {
			t.Fatalf("expected capability update mode %q, got %q", item.name, item.value)
		}
	}
}
