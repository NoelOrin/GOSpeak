package cluster

import (
	"errors"
	"fmt"
	"strings"
)

const (
	CommandKick         = "kick"
	CommandMute         = "mute"
	CommandUnmute       = "unmute"
	CommandDeleteRoom   = "delete_room"
	CommandDeleteServer = "delete_server"
	CommandKickDomain   = "kick_domain"
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

// Validate 校验控制命令的已知类型与必填字段。
func (c ControlCommand) Validate() error {
	if c.Command == "" {
		return errors.New("command is required")
	}
	switch c.Command {
	case CommandKick:
		if strings.TrimSpace(c.Room) == "" {
			return errors.New("kick requires room")
		}
		if strings.TrimSpace(c.Identity) == "" {
			return errors.New("kick requires identity")
		}
	case CommandDeleteRoom:
		if strings.TrimSpace(c.DomainUUID) == "" {
			return errors.New("delete_room requires domain_uuid")
		}
		if strings.TrimSpace(c.Room) == "" {
			return errors.New("delete_room requires room")
		}
	case CommandDeleteServer:
		if strings.TrimSpace(c.DomainUUID) == "" {
			return errors.New("delete_server requires domain_uuid")
		}
	case CommandKickDomain:
		if strings.TrimSpace(c.DomainUUID) == "" {
			return errors.New("kick_domain requires domain_uuid")
		}
		if !controlPayloadHasUserUUID(c.Payload) {
			return errors.New("kick_domain requires payload.user_uuid")
		}
	case CommandMute, CommandUnmute:
		if !controlPayloadHasUserID(c.Payload) {
			return fmt.Errorf("%s requires payload.user_id", c.Command)
		}
	default:
		return fmt.Errorf("unsupported command %q", c.Command)
	}
	return nil
}

func controlPayloadHasUserID(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	switch v := payload["user_id"].(type) {
	case float64:
		return v >= 0
	case int:
		return v >= 0
	case int64:
		return v >= 0
	case uint:
		return true
	}
	return false
}

func controlPayloadHasUserUUID(payload map[string]interface{}) bool {
	if payload == nil {
		return false
	}
	v, ok := payload["user_uuid"].(string)
	return ok && strings.TrimSpace(v) != ""
}
