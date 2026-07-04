package handler

import (
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/service"
	"GOSpeak/internal/signal"
	"time"

	"github.com/gin-gonic/gin"
)

// MuteBroadcaster 禁言事件广播接口，由 signal.Hub 实现
type MuteBroadcaster interface {
	BroadcastMute(userID uint, info *signal.MuteInfo)
	BroadcastUnmute(userID uint)
}

type MuteHandler struct {
	muteSvc     *service.MuteService
	userSvc     *service.UserService
	broadcaster MuteBroadcaster
}

func NewMuteHandler(muteSvc *service.MuteService, userSvc *service.UserService, broadcaster MuteBroadcaster) *MuteHandler {
	return &MuteHandler{muteSvc: muteSvc, userSvc: userSvc, broadcaster: broadcaster}
}

// CreateMute
// @Summary      禁言用户
// @Description  对用户进行定时或永久禁言
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{user_id=uint,duration=int64,permanent=bool,reason=string}  true  "禁言参数"
// @Success      200      {object}  pkg.Response
// @Router       /mute/create [post]
func (h *MuteHandler) CreateMute(c *gin.Context) {
	var req struct {
		UserID    uint   `json:"user_id" binding:"required"`
		Duration  int64  `json:"duration"`
		Permanent bool   `json:"permanent"`
		Reason    string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if !req.Permanent && req.Duration <= 0 {
		pkg.Fail(c, pkg.INVALID_PARAMS, "duration is required for non-permanent mute")
		return
	}

	userUUID, _ := c.Get("user_uuid")
	muter, err := h.userSvc.GetByUUID(userUUID.(string))
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	mute, err := h.muteSvc.MuteUser(muter.ID, req.UserID, req.Duration, req.Permanent, req.Reason)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	// 广播禁言事件 — convert model.Mute to signal.MuteInfo at the boundary
	if h.broadcaster != nil {
		info := &signal.MuteInfo{
			UserID:    mute.UserID,
			Duration:  mute.Duration,
			Permanent: mute.Permanent,
			Reason:    mute.Reason,
		}
		if mute.ExpiresAt != nil {
			info.ExpiresAt = mute.ExpiresAt.Format(time.RFC3339)
		}
		h.broadcaster.BroadcastMute(req.UserID, info)
	}

	pkg.Success(c, mute)
}

// CancelMute
// @Summary      取消禁言
// @Description  取消用户的禁言
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{user_id=uint}  true  "用户 ID"
// @Success      200     {object}  pkg.Response
// @Router       /mute/cancel [post]
func (h *MuteHandler) CancelMute(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	if err := h.muteSvc.UnmuteUser(req.UserID); err != nil {
		pkg.HandleError(c, err)
		return
	}

	// 广播取消禁言事件
	if h.broadcaster != nil {
		h.broadcaster.BroadcastUnmute(req.UserID)
	}

	pkg.Success(c, nil)
}

// GetMuteStatus
// @Summary      查询禁言状态
// @Description  查询指定用户是否被禁言
// @Tags         禁言
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{user_id=uint}  true  "用户 ID"
// @Success      200     {object}  pkg.Response
// @Router       /mute/status [post]
func (h *MuteHandler) GetMuteStatus(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	mute, err := h.muteSvc.GetMuteStatus(req.UserID)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, mute)
}

// ListMutes
// @Summary      禁言列表
// @Description  获取所有生效禁言记录
// @Tags         禁言
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  pkg.Response
// @Router       /mute/list [post]
func (h *MuteHandler) ListMutes(c *gin.Context) {
	mutes, err := h.muteSvc.ListActiveMutes()
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, mutes)
}
