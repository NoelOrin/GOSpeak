package signal

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/pkg/message"
	"GOSpeak/internal/policy"
	"GOSpeak/internal/ws"
)

// conversationSender abstracts private chat operations for the signal layer.
// Satisfied by *service.ConversationService.
type conversationSender interface {
	SendDirect(senderIdentity, targetIdentity, content, clientNonce string) (*message.DTO, error)
}

// privateSendPayload is the client->server payload for private:send.
type privateSendPayload struct {
	TargetIdentity string `json:"target_identity"`
	Content        string `json:"content"`
	ClientNonce    string `json:"client_nonce,omitempty"`
}

// OnPrivateMessageSend handles private:send events from clients.
func (h *Hub) OnPrivateMessageSend(c ws.ClientMessenger, data string) (string, error) {
	if h.convSvc == nil {
		return marshalAck(map[string]interface{}{"error": "conversation service unavailable"})
	}

	var req privateSendPayload
	if err := parseJSON(data, &req); err != nil || req.TargetIdentity == "" || req.Content == "" {
		return marshalAck(map[string]interface{}{"error": "target_identity and content are required"})
	}

	identity := clientIdentity(c)
	if identity == "" {
		return marshalAck(map[string]interface{}{"error": "unauthorized"})
	}

	// 与 HTTP 侧 BanCheck 复用同一份策略判决：/ws 绕过 Gin 中间件链，
	// 逐事件复查才能拦住封禁后仍持有长连接的身份。
	// payload 不带 domain_uuid，域上下文取服务端登记的连接当前域。
	if d := h.checkWSPolicy(c, policy.Action(EventPrivateSend), h.ClientDomain(c.ID())); !d.Allowed {
		return marshalPolicyAck(d), nil
	}

	dto, err := h.convSvc.SendDirect(identity, req.TargetIdentity, req.Content, req.ClientNonce)
	if err != nil {
		_, msg := pkg.ClientError(err)
		return marshalAck(map[string]interface{}{"error": msg})
	}

	return marshalAck(dto)
}
