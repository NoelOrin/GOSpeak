package handler

import (
	"go_rtc/internal/model"
	"go_rtc/internal/pkg"
	"go_rtc/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	roomSvc *service.RoomService
	permSvc *service.PermissionService
}

func NewRoomHandler(roomSvc *service.RoomService, permSvc *service.PermissionService) *RoomHandler {
	return &RoomHandler{roomSvc: roomSvc, permSvc: permSvc}
}

type CreateRoomRequest struct {
	Name  string `json:"name" binding:"required"`
	Limit uint   `json:"limit"`
}

// Create
// @Summary      创建房间
// @Description  创建新房间
// @Tags         房间
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      CreateRoomRequest  true  "房间信息"
// @Success      200      {object}  pkg.Response
// @Router       /room/create [post]
func (h *RoomHandler) Create(c *gin.Context) {
	var req CreateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	username, _ := c.Get("username")
	room := &model.Room{
		Name:      req.Name,
		Limit:     req.Limit,
		CreatedBy: username.(string),
	}
	if err := h.roomSvc.Create(room); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, room)
}

// Get
// @Summary      获取房间详情
// @Description  根据 ID 获取房间信息
// @Tags         房间
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "房间 ID"
// @Success      200  {object}  pkg.Response
// @Router       /room/{id} [get]
func (h *RoomHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	room, err := h.roomSvc.GetByID(uint(id))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, room)
}

// List
// @Summary      房间列表
// @Description  分页获取房间列表
// @Tags         房间
// @Produce      json
// @Security     BearerAuth
// @Param        page       query     int  false  "页码"      default(1)
// @Param        page_size  query     int  false  "每页条数"  default(20)
// @Success      200        {object}  pkg.Response
// @Router       /room/list [get]
func (h *RoomHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	rooms, total, err := h.roomSvc.List(page, pageSize)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{
		"rooms": rooms,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

type UpdateRoomRequest struct {
	Name  string `json:"name"`
	Limit uint   `json:"limit"`
}

// Update
// @Summary      更新房间
// @Description  根据 ID 更新房间信息
// @Tags         房间
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int               true  "房间 ID"
// @Param        request  body      UpdateRoomRequest  true  "更新内容"
// @Success      200      {object}  pkg.Response
// @Router       /room/{id} [put]
func (h *RoomHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	var req UpdateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	room, err := h.roomSvc.GetByID(uint(id))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	// 资源归属校验：非 room:update 权限的用户只能编辑自己创建的房间
	username, _ := c.Get("username")
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	if !h.permSvc.HasPermission(roleStr, model.PermRoomUpdate) {
		if room.CreatedBy != username.(string) {
			pkg.Fail(c, pkg.FORBIDDEN, "只能编辑自己创建的房间")
			return
		}
	}

	if req.Name != "" {
		room.Name = req.Name
	}
	room.Limit = req.Limit

	if err := h.roomSvc.Update(room); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, room)
}

// Delete
// @Summary      删除房间
// @Description  根据 ID 删除房间
// @Tags         房间
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "房间 ID"
// @Success      200  {object}  pkg.Response
// @Router       /room/{id} [delete]
func (h *RoomHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, "invalid id")
		return
	}

	if err := h.roomSvc.Delete(uint(id)); err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, nil)
}
