package handler

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/permcode"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
)

type AnnouncementHandler struct {
	svc     *service.AnnouncementService
	permSvc *service.PermissionService
}

func NewAnnouncementHandler(svc *service.AnnouncementService, permSvc *service.PermissionService) *AnnouncementHandler {
	return &AnnouncementHandler{svc: svc, permSvc: permSvc}
}

func (h *AnnouncementHandler) actor(c *gin.Context) (service.AnnouncementActor, bool) {
	userUUID := currentUserUUID(c)
	if userUUID == "" {
		return service.AnnouncementActor{}, false
	}
	nameVal, _ := c.Get("username")
	name, _ := nameVal.(string)
	return service.AnnouncementActor{
		UserUUID:        userUUID,
		Name:            name,
		IsPlatformAdmin: h.permSvc != nil && h.permSvc.HasPermission(roleFromContext(c), permcode.PermAnnouncementManage),
	}, true
}

// List
// @Summary      公告列表
// @Description  返回当前用户可见的系统公告，含已读状态
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{only_unread=bool,page=int,page_size=int}  false  "查询条件"
// @Success      200      {object}  pkg.Response
// @Router       /announcement/list [post]
func (h *AnnouncementHandler) List(c *gin.Context) {
	var req struct {
		OnlyUnread bool `json:"only_unread"`
		Page       int  `json:"page"`
		PageSize   int  `json:"page_size"`
	}
	_ = c.ShouldBindJSON(&req)

	items, total, err := h.svc.List(currentUserUUID(c), req.OnlyUnread, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "total": total})
}

// UnreadCount
// @Summary      公告未读数
// @Description  返回当前用户可见但未读的公告数量
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /announcement/unread-count [post]
func (h *AnnouncementHandler) UnreadCount(c *gin.Context) {
	count, err := h.svc.UnreadCount(currentUserUUID(c))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"count": count})
}

// MarkRead
// @Summary      标记公告已读
// @Description  幂等标记单条公告已读；公告不可见时返回 NOT_FOUND
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{uuid=string}  true  "公告 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/mark-read [post]
func (h *AnnouncementHandler) MarkRead(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if err := h.svc.MarkRead(currentUserUUID(c), req.UUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"ok": true})
}

// AdminList
// @Summary      管理端公告列表
// @Description  平台管理员看全部；域管理员只看自己可管理的域公告与自己的草稿
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{page=int,page_size=int}  false  "分页"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/admin/list [post]
func (h *AnnouncementHandler) AdminList(c *gin.Context) {
	var req struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	_ = c.ShouldBindJSON(&req)

	actor, ok := h.actor(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	if !actor.IsPlatformAdmin && !h.hasAnyDomainManage(c) {
		pkg.Fail(c, pkg.FORBIDDEN)
		return
	}
	items, total, err := h.svc.ListAdmin(actor, req.Page, req.PageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"items": items, "total": total})
}

func (h *AnnouncementHandler) hasAnyDomainManage(c *gin.Context) bool {
	if h.permSvc == nil {
		return false
	}
	return h.permSvc.HasPermission(roleFromContext(c), permcode.PermDomainManage)
}

type announcementInputBody struct {
	ScopeType string `json:"scope_type"`
	ScopeID   string `json:"scope_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Level     string `json:"level"`
	Pinned    bool   `json:"pinned"`
	Publish   *bool  `json:"publish"`
	ExpiresAt *int64 `json:"expires_at"`
}

func (b announcementInputBody) toInput() service.AnnouncementInput {
	return service.AnnouncementInput{
		ScopeType: b.ScopeType,
		ScopeID:   b.ScopeID,
		Title:     b.Title,
		Content:   b.Content,
		Level:     b.Level,
		Pinned:    b.Pinned,
		Publish:   b.Publish,
		ExpiresAt: b.ExpiresAt,
	}
}

// AdminCreate
// @Summary      创建公告
// @Description  平台公告需 announcement:manage；域公告需该域 domain:manage
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{scope_type=string,scope_id=string,title=string,content=string,level=string,pinned=bool,publish=bool,expires_at=int64}  true  "公告内容"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/admin/create [post]
func (h *AnnouncementHandler) AdminCreate(c *gin.Context) {
	var req announcementInputBody
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	if req.ScopeType == "" {
		req.ScopeType = model.AnnouncementScopePlatform
	}
	actor, ok := h.actor(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	dto, err := h.svc.Create(actor, req.toInput())
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, dto)
}

// AdminUpdate
// @Summary      更新公告
// @Description  按公告 scope 判定权限；草稿首次发布时投递
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{uuid=string,title=string,content=string,level=string,pinned=bool,publish=bool,expires_at=int64}  true  "公告内容"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/admin/update [post]
func (h *AnnouncementHandler) AdminUpdate(c *gin.Context) {
	var req struct {
		announcementInputBody
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	dto, err := h.svc.Update(actor, req.UUID, req.toInput())
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, dto)
}

// AdminDelete
// @Summary      删除公告
// @Description  软删除公告
// @Tags         公告
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{uuid=string}  true  "公告 UUID"
// @Success      200  {object}  pkg.Response
// @Router       /announcement/admin/delete [post]
func (h *AnnouncementHandler) AdminDelete(c *gin.Context) {
	var req struct {
		UUID string `json:"uuid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}
	actor, ok := h.actor(c)
	if !ok {
		pkg.Fail(c, pkg.INVALID_PARAMS, "not authenticated")
		return
	}
	if err := h.svc.Delete(actor, req.UUID); err != nil {
		pkg.HandleError(c, err)
		return
	}
	pkg.Success(c, gin.H{"ok": true})
}
