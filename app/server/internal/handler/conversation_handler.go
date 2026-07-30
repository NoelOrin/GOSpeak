package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	svc *service.ConversationService
}

func NewConversationHandler(svc *service.ConversationService) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

func (h *ConversationHandler) List(c *gin.Context) {
	var req struct {
		Limit int `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username, exists := c.Get("username")
	if !exists {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	identity, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid context")
		return
	}

	out, err := h.svc.List(identity, req.Limit)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}

func (h *ConversationHandler) Messages(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversation_id" binding:"required"`
		Before         string `json:"before"`
		Limit          int    `json:"limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username, exists := c.Get("username")
	if !exists {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	identity, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid context")
		return
	}

	out, err := h.svc.GetMessages(req.ConversationID, identity, req.Before, req.Limit)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}

func (h *ConversationHandler) MarkRead(c *gin.Context) {
	var req struct {
		ConversationID string `json:"conversation_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username, exists := c.Get("username")
	if !exists {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	identity, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid context")
		return
	}

	if err := h.svc.MarkRead(req.ConversationID, identity); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

func (h *ConversationHandler) Send(c *gin.Context) {
	var req struct {
		TargetIdentity string `json:"target_identity" binding:"required"`
		Content        string `json:"content" binding:"required"`
		ClientNonce    string `json:"client_nonce"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username, exists := c.Get("username")
	if !exists {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	identity, ok := username.(string)
	if !ok {
		pkg.Fail(c, pkg.INTERNAL_ERROR, "invalid context")
		return
	}

	out, err := h.svc.SendDirect(identity, req.TargetIdentity, req.Content, req.ClientNonce)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, out)
}
