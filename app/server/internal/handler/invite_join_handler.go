package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// InviteJoinHandler 内联注册、加入、申请查询与撤销。
type InviteJoinHandler struct {
	joinSvc *service.InviteJoinService
	cookie  *AuthCookieConfig
}

func NewInviteJoinHandler(joinSvc *service.InviteJoinService, cookie *AuthCookieConfig) *InviteJoinHandler {
	if cookie == nil {
		cookie = defaultAuthCookieConfig()
	}
	return &InviteJoinHandler{joinSvc: joinSvc, cookie: cookie}
}

// Register 内联注册：验码 → 建用户 → 按域策略入域或提交申请 → 签发 Cookie。
// @Summary      邀请码内联注册
// @Tags         邀请
// @Accept       json
// @Produce      json
// @Param        request  body  service.InviteRegisterRequest  true  "邀请码与注册信息"
// @Success      200  {object}  pkg.Response
// @Router       /invite/register [post]
func (h *InviteJoinHandler) Register(c *gin.Context) {
	var req service.InviteRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.joinSvc.RegisterAndJoin(&req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	h.cookie.Set(c, resp.Token, resp.RefreshToken)
	pkg.Success(c, resp)
}

type inviteJoinRequest struct {
	Code       string `json:"code"`
	DomainUUID string `json:"domain_uuid"`
}

// Join 已登录用户加入域（邀请码或公开域免码）。
// @Summary      加入域
// @Tags         邀请
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  inviteJoinRequest  true  "邀请码或域名 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /invite/join [post]
func (h *InviteJoinHandler) Join(c *gin.Context) {
	var req inviteJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.joinSvc.Join(currentUserUUID(c), &service.InviteJoinRequest{
		Code: req.Code, DomainUUID: req.DomainUUID,
	})
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, resp)
}

type domainUUIDOnlyRequest struct {
	DomainUUID string `json:"domain_uuid" binding:"required"`
}

// MyRequest 查询自己在该域的申请状态。
// @Summary      查询加入申请状态
// @Tags         邀请
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  domainUUIDOnlyRequest  true  "域名 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /invite/my-request [post]
func (h *InviteJoinHandler) MyRequest(c *gin.Context) {
	var req domainUUIDOnlyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	view, err := h.joinSvc.MyRequest(currentUserUUID(c), req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, view)
}

// CancelRequest 撤销自己的待审申请。
// @Summary      撤销加入申请
// @Tags         邀请
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  domainUUIDOnlyRequest  true  "域名 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /invite/cancel-request [post]
func (h *InviteJoinHandler) CancelRequest(c *gin.Context) {
	var req domainUUIDOnlyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.joinSvc.CancelRequest(currentUserUUID(c), req.DomainUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"ok": true})
}
