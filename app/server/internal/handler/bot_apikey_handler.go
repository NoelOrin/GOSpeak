package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type BotAPIKeyHandler struct {
	svc *service.BotAPIKeyService
}

func NewBotAPIKeyHandler(svc *service.BotAPIKeyService) *BotAPIKeyHandler {
	return &BotAPIKeyHandler{svc: svc}
}

// Create
// @Summary      生成 Bot API Key
// @Description  创建具备受限权限与过期时间的 Bot 专用 API Key，明文 key 仅返回一次
// @Tags         Bot 密钥
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body service.CreateBotKeyRequest true "Bot Key 配置"
// @Success      200 {object} pkg.Response
// @Router       /bot/key/create [post]
func (h *BotAPIKeyHandler) Create(c *gin.Context) {
	username, _ := c.Get("username")
	creator, _ := username.(string)

	var req service.CreateBotKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	result, err := h.svc.Create(req, creator)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{
		"key":       result.Key,
		"plain_key": result.PlainKey,
	})
}

// List
// @Summary      列出 Bot API Key
// @Description  列出当前用户创建的 Bot Key（管理员可见全部）
// @Tags         Bot 密钥
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} pkg.Response
// @Router       /bot/key/list [post]
func (h *BotAPIKeyHandler) List(c *gin.Context) {
	username, _ := c.Get("username")
	creator, _ := username.(string)

	keys, err := h.svc.List(creator)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, keys)
}

// Revoke
// @Summary      吊销 Bot API Key
// @Description  吊销指定 UUID 的 Bot Key
// @Tags         Bot 密钥
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body object{uuid=string} true "Key UUID"
// @Success      200 {object} pkg.Response
// @Router       /bot/key/revoke [post]
func (h *BotAPIKeyHandler) Revoke(c *gin.Context) {
	username, _ := c.Get("username")
	creator, _ := username.(string)

	var req struct {
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.svc.Revoke(req.UUID, creator); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}
