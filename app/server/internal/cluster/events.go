package cluster

const (
	EventNodeRegistered   = "cluster.node.registered"
	EventNodeDeregistered = "cluster.node.deregistered"
	EventNodeDraining     = "cluster.node.draining"
	EventNodeUndrained    = "cluster.node.undrained"
	EventNodeHeartbeat    = "cluster.node.heartbeat"
	EventServerScaled     = "cluster.server.scaled"
	EventServerDeleted    = "cluster.server.deleted"
)
