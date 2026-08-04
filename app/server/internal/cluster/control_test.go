package cluster

import "testing"

func TestControlCommandValidate(t *testing.T) {
	cmd := ControlCommand{Command: CommandKick, NodeID: "node-a", Room: "lobby", Identity: "alice"}
	if err := cmd.Validate(); err != nil {
		t.Fatalf("expected valid command, got %v", err)
	}
}

func TestControlCommandValidateMissingFields(t *testing.T) {
	if err := (ControlCommand{NodeID: "node-a"}).Validate(); err == nil {
		t.Fatal("expected missing command error")
	}
}
