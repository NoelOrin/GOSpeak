package signal

import (
	"testing"

	"GOSpeak/internal/cluster"
)

func TestHubHandleClusterCommandKick(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: cluster.CommandKick, Room: "lobby", Identity: "alice"}
	if err := hub.HandleClusterCommand(cmd); err != nil {
		t.Fatalf("HandleClusterCommand: %v", err)
	}
}

func TestHubHandleClusterCommandDeleteRoom(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: cluster.CommandDeleteRoom, DomainUUID: "domain-a", Room: "lobby"}
	if err := hub.HandleClusterCommand(cmd); err != nil {
		t.Fatalf("HandleClusterCommand: %v", err)
	}
}

func TestHubHandleClusterCommandDeleteServer(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: cluster.CommandDeleteServer, DomainUUID: "domain-a"}
	if err := hub.HandleClusterCommand(cmd); err != nil {
		t.Fatalf("HandleClusterCommand: %v", err)
	}
}

func TestHubHandleClusterCommandUnsupported(t *testing.T) {
	hub := NewHub(nil, nil, nil, nil)
	cmd := cluster.ControlCommand{Command: "nope", NodeID: "node-a"}
	if err := hub.HandleClusterCommand(cmd); err == nil {
		t.Fatal("expected unsupported command error")
	}
}
