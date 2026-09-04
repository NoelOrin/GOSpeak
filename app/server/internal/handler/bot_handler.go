package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type BotHandler struct {
	svc *service.BotService
}

func NewBotHandler(svc *service.BotService) *BotHandler {
	return &BotHandler{svc: svc}
}

// Create
// @Summary      创建 Bot
// @Description  创建 Bot 用户并签发 JWT。expires_in 为空时生成永久 token（100年）
// @Tags         Bot
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body service.CreateBotRequest true "Bot 配置"
// @Success      200 {object} pkg.Response{data=service.CreateBotResult}
// @Router       /bot/create [post]
func (h *BotHandler) Create(c *gin.Context) {
	var req service.CreateBotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	result, err := h.svc.Create(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, result)
}

// List
// @Summary      列出 Bot
// @Description  列出所有 Bot Token 管理记录
// @Tags         Bot
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} pkg.Response
// @Router       /bot/list [post]
func (h *BotHandler) List(c *gin.Context) {
	tokens, err := h.svc.List()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, tokens)
}

// Revoke
// @Summary      吊销 Bot
// @Description  吊销指定 Bot Token（标记 revoked=true）
// @Tags         Bot
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body object{uuid=string} true "Bot UUID"
// @Success      200 {object} pkg.Response
// @Router       /bot/revoke [post]
func (h *BotHandler) Revoke(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.svc.Revoke(req.UUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
