package cluster

import "testing"

func TestControlCommandValidate(t *testing.T) {
	valid := []ControlCommand{
		{Command: CommandKick, NodeID: "node-a", Room: "lobby", Identity: "alice"},
		{Command: CommandDeleteRoom, DomainUUID: "domain-a", Room: "lobby"},
		{Command: CommandDeleteServer, DomainUUID: "domain-a"},
		{Command: CommandMute, Payload: map[string]interface{}{"user_id": uint(1)}},
		{Command: CommandUnmute, Payload: map[string]interface{}{"user_id": uint(1)}},
	}
	for _, cmd := range valid {
		if err := cmd.Validate(); err != nil {
			t.Fatalf("expected valid command %+v, got %v", cmd, err)
		}
	}
}

func TestControlCommandValidateRejectsUnknownAndMissingFields(t *testing.T) {
	invalid := []ControlCommand{
		{NodeID: "node-a"},
		{Command: "nope", NodeID: "node-a"},
		{Command: CommandKick, NodeID: "node-a", Identity: "alice"},
		{Command: CommandKick, NodeID: "node-a", Room: "lobby"},
		{Command: CommandDeleteRoom, DomainUUID: "domain-a"},
		{Command: CommandDeleteRoom, Room: "lobby"},
		{Command: CommandDeleteServer, Room: "lobby"},
		{Command: CommandMute},
		{Command: CommandUnmute, Payload: map[string]interface{}{}},
	}
	for _, cmd := range invalid {
		if err := cmd.Validate(); err == nil {
			t.Fatalf("expected validation error for %+v", cmd)
		}
	}
}
