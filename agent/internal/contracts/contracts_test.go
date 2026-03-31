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
}
