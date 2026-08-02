package cluster

import (
	"testing"

	"GOSpeak/internal/model"
)

func TestChooseNodesSkipsUnhealthyAndPrefersPreferred(t *testing.T) {
	nodes := []model.ClusterNode{
		{UUID: "a", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
		{UUID: "b", Status: model.ClusterNodeBusy, SFUHealthy: true, MaxServers: 10, MaxRooms: 100, LoadPercent: 90},
		{UUID: "c", Status: model.ClusterNodeDraining, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
		{UUID: "d", Status: model.ClusterNodeReady, SFUHealthy: false, MaxServers: 10, MaxRooms: 100},
	}

	got := ChooseNodes(nodes, nil, 2, "a")
	if len(got) != 2 {
		t.Fatalf("expected 2 nodes, got %v", got)
	}
	if got[0] != "a" {
		t.Fatalf("expected preferred node first, got %v", got)
	}
	if got[1] != "b" {
		t.Fatalf("expected healthy busy node second, got %v", got)
	}
}

func TestChooseNodesSkipsAlreadyAssigned(t *testing.T) {
	nodes := []model.ClusterNode{
		{UUID: "a", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
		{UUID: "b", Status: model.ClusterNodeReady, SFUHealthy: true, MaxServers: 10, MaxRooms: 100},
	}
	current := []model.ServerAssignment{{ServerUUID: "srv", NodeUUID: "a"}}

	got := ChooseNodes(nodes, current, 1)
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("expected only unassigned node b, got %v", got)
	}
}

func TestChooseNodesZeroCount(t *testing.T) {
	if got := ChooseNodes(nil, nil, 0); got != nil {
		t.Fatalf("expected nil for zero count, got %v", got)
	}
}
