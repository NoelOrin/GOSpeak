package handler

import (
	"fmt"
	"log"

	"GOSpeak/internal/audit"
	"GOSpeak/internal/cluster"
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

// ControlPublisher 发布集群控制命令，由目标 Worker 执行本地信令操作。
type ControlPublisher interface {
	PublishControl(cluster.ControlCommand) error
}

type MuteHandler struct {
	muteSvc          *service.MuteService
	userSvc          *service.UserService
	broadcaster      MuteBroadcaster
	controlPublisher ControlPublisher
	auditor          *audit.Service
}

func NewMuteHandler(muteSvc *service.MuteService, userSvc *service.UserService, broadcaster MuteBroadcaster) *MuteHandler {
	return &MuteHandler{muteSvc: muteSvc, userSvc: userSvc, broadcaster: broadcaster}
}

// SetControlPublisher 注入集群控制命令发布器。
func (h *MuteHandler) SetControlPublisher(p ControlPublisher) {
	h.controlPublisher = p
}

// SetAuditor 注入审计服务，用于记录禁言/解禁操作。
func (h *MuteHandler) SetAuditor(a *audit.Service) { h.auditor = a }

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

	userUUID, ok := c.Get("user_uuid")
	if !ok {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST, "missing user identity in context")
		return
	}
	userUUIDStr, ok := userUUID.(string)
	if !ok || userUUIDStr == "" {
		pkg.Fail(c, pkg.TOKEN_NOT_EXIST, "invalid user identity in context")
		return
	}
	muter, err := h.userSvc.GetByUUID(userUUIDStr)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	mute, err := h.muteSvc.MuteUser(muter.ID, req.UserID, req.Duration, req.Permanent, req.Reason)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}
	if h.auditor != nil {
		h.auditor.Log(audit.Entry{
			ActorID:    muter.ID,
			ActorUUID:  muter.UUID,
			ActorName:  muter.Name,
			Action:     audit.ActionMuteUser,
			TargetType: audit.TargetUser,
			TargetID:   fmt.Sprintf("%d", req.UserID),
			Detail:     fmt.Sprintf("permanent=%v duration=%ds reason=%q", req.Permanent, req.Duration, req.Reason),
			IP:         audit.AuditIP(c),
			Success:    true,
		})
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

	// DB 与广播已生效；控制命令失败只告警，避免客户端收到 5xx 却看到状态已变更。
	if h.controlPublisher != nil {
		if err := h.controlPublisher.PublishControl(cluster.ControlCommand{
			Command:  cluster.CommandMute,
			Identity: userUUIDStr,
			Payload: map[string]interface{}{
				"user_id": req.UserID, "permanent": req.Permanent, "duration": req.Duration, "reason": req.Reason,
			},
		}); err != nil {
			log.Printf("[Mute] publish control mute failed user=%d err=%v", req.UserID, err)
		}
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
	if h.auditor != nil {
		ua, un := auditActor(c)
		h.auditor.Log(audit.Entry{
			ActorUUID:  ua,
			ActorName:  un,
			Action:     audit.ActionUnmuteUser,
			TargetType: audit.TargetUser,
			TargetID:   fmt.Sprintf("%d", req.UserID),
			IP:         audit.AuditIP(c),
			Success:    true,
		})
	}

	// 广播取消禁言事件
	if h.broadcaster != nil {
		h.broadcaster.BroadcastUnmute(req.UserID)
	}

	if h.controlPublisher != nil {
		if err := h.controlPublisher.PublishControl(cluster.ControlCommand{
			Command: cluster.CommandUnmute,
			Payload: map[string]interface{}{"user_id": req.UserID},
		}); err != nil {
			log.Printf("[Mute] publish control unmute failed user=%d err=%v", req.UserID, err)
		}
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
