package cluster

import "errors"

const (
	CommandKick         = "kick"
	CommandMute         = "mute"
	CommandUnmute       = "unmute"
	CommandDeleteRoom   = "delete_room"
	CommandDeleteServer = "delete_server"
)

// ControlCommand 是 Agent 通过 NATS internal 事件下发给 Worker 的控制命令。
type ControlCommand struct {
	Command    string                 `json:"command"`
	NodeID     string                 `json:"node_id,omitempty"`
	DomainUUID string                 `json:"domain_uuid,omitempty"`
	Room       string                 `json:"room,omitempty"`
	Identity   string                 `json:"identity,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// Validate 校验控制命令必填字段。
func (c ControlCommand) Validate() error {
	if c.Command == "" {
		return errors.New("command is required")
	}
	return nil
}
