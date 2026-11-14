package signal

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/protocol"
	"GOSpeak/internal/pkg/message"
	"GOSpeak/internal/policy"
	"GOSpeak/internal/ws"
)

// conversationSender abstracts private chat operations for the signal layer.
// Satisfied by *service.ConversationService.
type conversationSender interface {
	SendDirect(senderIdentity, targetIdentity, content, clientNonce string) (*message.DTO, error)
}

// privateSendPayload 定义已收编至 protocol（PrivateSendPayload）。
type privateSendPayload = protocol.PrivateSendPayload

// OnPrivateMessageSend handles private:send events from clients.
// ACK 约定：成功 data = 私聊消息 DTO；失败走 error 返回路径（*AppError）。
func (h *Hub) OnPrivateMessageSend(c ws.ClientMessenger, data string) (string, error) {
	if h.convSvc == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "conversation service unavailable")
	}

	var req privateSendPayload
	if err := parseJSON(data, &req); err != nil || req.TargetIdentity == "" || req.Content == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "target_identity and content are required")
	}

	identity := clientIdentity(c)
	if identity == "" {
		return "", pkg.NewAppError(pkg.FORBIDDEN, "unauthorized")
	}

	// 与 HTTP 侧 BanCheck 复用同一份策略判决：/ws 绕过 Gin 中间件链，
	// 逐事件复查才能拦住封禁后仍持有长连接的身份。
	// payload 不带 domain_uuid，域上下文取服务端登记的连接当前域。
	if d := h.checkWSPolicy(c, policy.Action(EventPrivateSend), h.ClientDomain(c.ID())); !d.Allowed {
		msg := d.Message
		if msg == "" {
			msg = pkg.GetErrMsg(d.Code)
		}
		return "", pkg.NewAppError(d.Code, msg)
	}

	dto, err := h.convSvc.SendDirect(identity, req.TargetIdentity, req.Content, req.ClientNonce)
	if err != nil {
		return "", err
	}
	return marshalAck(dto)
}
