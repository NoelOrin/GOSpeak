package signal

import (
	"GOSpeak/internal/pkg/message"
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

	dto, err := h.convSvc.SendDirect(identity, req.TargetIdentity, req.Content, req.ClientNonce)
	if err != nil {
		return marshalAck(map[string]interface{}{"error": err.Error()})
	}

	return marshalAck(dto)
}
