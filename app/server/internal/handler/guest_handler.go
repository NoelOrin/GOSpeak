package handler

import (
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

// GuestHandler 访客访问相关 HTTP 接口。
type GuestHandler struct {
	guestSvc     *service.GuestService
	domainSvc    domainPermissionChecker
	permSvc      globalPermissionChecker
	cookie       *AuthCookieConfig
	onDomainKick func(domainUUID, userUUID string)
}

// NewGuestHandler 构造 GuestHandler；domainSvc/permSvc 为管理端接口的权限判决依赖。
func NewGuestHandler(
	guestSvc *service.GuestService,
	domainSvc domainPermissionChecker,
	permSvc globalPermissionChecker,
	cookie *AuthCookieConfig,
) *GuestHandler {
	if cookie == nil {
		cookie = defaultAuthCookieConfig()
	}
	return &GuestHandler{guestSvc: guestSvc, domainSvc: domainSvc, permSvc: permSvc, cookie: cookie}
}

// SetOnDomainKick 注入封禁后的信令踢线回调。
func (h *GuestHandler) SetOnDomainKick(fn func(domainUUID, userUUID string)) {
	h.onDomainKick = fn
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

// guestConfigRequest 域访客配置读写请求；除 domain_uuid 外字段可选，
// 全部缺省表示读取。
type guestConfigRequest struct {
	DomainUUID      string `json:"domain_uuid" binding:"required"`
	AllowGuest      *bool  `json:"allow_guest"`
	GuestCanListen  *bool  `json:"guest_can_listen"`
	GuestCanSpeak   *bool  `json:"guest_can_speak"`
	GuestCanMessage *bool  `json:"guest_can_message"`
	GuestLimit      *int   `json:"guest_limit"`
}

func (h *GuestHandler) hasWritePerm(c *gin.Context, dom, perm string) bool {
	return domainPermissionGranted(c, dom, perm, h.domainSvc, h.permSvc)
}

// Config
// @Summary      读写域访客配置
// @Tags         Domain
// @Router       /domain/guest/config [post]
func (h *GuestHandler) Config(c *gin.Context) {
	var req guestConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.hasWritePerm(c, req.DomainUUID, permcode.PermDomainManage) {
		pkg.Fail(c, pkg.FORBIDDEN)
		return
	}
	if req.AllowGuest == nil && req.GuestCanListen == nil && req.GuestCanSpeak == nil &&
		req.GuestCanMessage == nil && req.GuestLimit == nil {
		domain, err := h.guestSvc.GetConfig(req.DomainUUID)
		if err != nil {
			pkg.HandleError(c, err)
			return
		}
		pkg.Success(c, domain)
		return
	}
	domain, err := h.guestSvc.UpdateConfig(req.DomainUUID, service.GuestConfigUpdate{
		AllowGuest:      req.AllowGuest,
		GuestCanListen:  req.GuestCanListen,
		GuestCanSpeak:   req.GuestCanSpeak,
		GuestCanMessage: req.GuestCanMessage,
		GuestLimit:      req.GuestLimit,
	})
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, domain)
}

type guestBanRequest struct {
	DomainUUID    string `json:"domain_uuid" binding:"required"`
	UserUUID      string `json:"user_uuid" binding:"required"`
	Reason        string `json:"reason"`
	DurationHours int    `json:"duration_hours"`
}

// Ban
// @Summary      封禁域内访客并踢下线
// @Tags         Domain
// @Router       /domain/guest/ban [post]
func (h *GuestHandler) Ban(c *gin.Context) {
	var req guestBanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.hasWritePerm(c, req.DomainUUID, permcode.PermDomainKick) {
		pkg.Fail(c, pkg.FORBIDDEN)
		return
	}
	if err := h.guestSvc.Ban(req.DomainUUID, currentUserUUID(c), req.UserUUID, req.Reason, req.DurationHours); err != nil {
		pkg.HandleError(c, err)
		return
	}
	if h.onDomainKick != nil {
		h.onDomainKick(req.DomainUUID, req.UserUUID)
	}
	pkg.Success(c, nil)
}

// Unban
// @Summary      解封域内访客
// @Tags         Domain
// @Router       /domain/guest/unban [post]
func (h *GuestHandler) Unban(c *gin.Context) {
	var req struct {
		DomainUUID string `json:"domain_uuid" binding:"required"`
		UserUUID   string `json:"user_uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.hasWritePerm(c, req.DomainUUID, permcode.PermDomainKick) {
		pkg.Fail(c, pkg.FORBIDDEN)
		return
	}
	if err := h.guestSvc.Unban(req.DomainUUID, req.UserUUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, nil)
}

// BanList
// @Summary      域访客封禁列表
// @Tags         Domain
// @Router       /domain/guest/ban-list [post]
func (h *GuestHandler) BanList(c *gin.Context) {
	var req struct {
		DomainUUID string `json:"domain_uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.hasWritePerm(c, req.DomainUUID, permcode.PermDomainManage) {
		pkg.Fail(c, pkg.FORBIDDEN)
		return
	}
	list, err := h.guestSvc.ListBans(req.DomainUUID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, list)
}

// Cleanup
// @Summary      清理 N 天未活跃访客
// @Tags         Domain
// @Router       /domain/guest/cleanup [post]
func (h *GuestHandler) Cleanup(c *gin.Context) {
	var req struct {
		DomainUUID string `json:"domain_uuid" binding:"required"`
		Days       int    `json:"days" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if !h.hasWritePerm(c, req.DomainUUID, permcode.PermDomainManage) {
		pkg.Fail(c, pkg.FORBIDDEN)
		return
	}
	count, err := h.guestSvc.CleanupInactiveGuests(req.Days)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"removed": count})
}

// Renew
// @Summary      已有访客身份加入另一个 Domain
// @Tags         认证
// @Router       /auth/guest/renew [post]
func (h *GuestHandler) Renew(c *gin.Context) {
	var req service.GuestJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	resp, err := h.guestSvc.Renew(currentUserUUID(c), &req)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	h.cookie.Set(c, resp.AccessToken, resp.RefreshToken)
	pkg.Success(c, resp)
}
