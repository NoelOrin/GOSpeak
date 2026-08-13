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

	got := ChooseNodesWithRequirement(nodes, nil, 2, []string{"a"}, NodeRequirement{})
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

	got := ChooseNodesWithRequirement(nodes, current, 1, nil, NodeRequirement{})
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("expected only unassigned node b, got %v", got)
	}
}

func TestChooseNodesZeroCount(t *testing.T) {
	if got := ChooseNodesWithRequirement(nil, nil, 0, nil, NodeRequirement{}); got != nil {
		t.Fatalf("expected nil for zero count, got %v", got)
	}
}

func TestChooseNodesWithRequirementFiltersLabelsAndProvider(t *testing.T) {
	nodes := []model.ClusterNode{
		{UUID: "livekit-cn", Status: model.ClusterNodeReady, SFUHealthy: true, SFUProvider: "livekit", MaxServers: 10, MaxRooms: 100},
		{UUID: "srs-cn", Status: model.ClusterNodeReady, SFUHealthy: true, SFUProvider: "srs", MaxServers: 10, MaxRooms: 100},
		{UUID: "livekit-global", Status: model.ClusterNodeReady, SFUHealthy: true, SFUProvider: "livekit", MaxServers: 10, MaxRooms: 100},
	}
	nodes[0].SetLabels(map[string]string{"region": "cn", "pool": "voice"})
	nodes[1].SetLabels(map[string]string{"region": "cn", "pool": "voice"})
	nodes[2].SetLabels(map[string]string{"region": "global", "pool": "voice"})

	req := NodeRequirement{
		SFUProvider: "livekit",
		Labels:      map[string]string{"region": "cn", "pool": "voice"},
	}
	got := ChooseNodesWithRequirement(nodes, nil, 2, nil, req)
	if len(got) != 1 || got[0] != "livekit-cn" {
		t.Fatalf("expected only livekit-cn, got %v", got)
	}
}
