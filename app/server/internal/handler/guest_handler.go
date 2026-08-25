package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// GuestHandler 访客访问相关 HTTP 接口。
type GuestHandler struct {
	guestSvc *service.GuestService
	cookie   *AuthCookieConfig
}

// NewGuestHandler 构造 GuestHandler。
func NewGuestHandler(guestSvc *service.GuestService, cookie *AuthCookieConfig) *GuestHandler {
	if cookie == nil {
		cookie = defaultAuthCookieConfig()
	}
	return &GuestHandler{guestSvc: guestSvc, cookie: cookie}
}

// Join
// @Summary      访客加入 Domain
// @Description  以匿名访客身份签发标准 JWT 并加入指定 Domain（邀请码或公开域）
// @Tags         认证
// @Accept       json
// @Produce      json
// @Param        request  body      service.GuestJoinRequest  true  "访客昵称与加入方式"
// @Success      200      {object}  pkg.Response
// @Router       /auth/guest [post]
func (h *GuestHandler) Join(c *gin.Context) {
	var req service.GuestJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.guestSvc.Join(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	h.cookie.Set(c, resp.AccessToken, resp.RefreshToken)
	pkg.Success(c, resp)
}
