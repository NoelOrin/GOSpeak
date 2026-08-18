package handler

import (
	"time"

	"GOSpeak/internal/audit"
	"GOSpeak/internal/pkg"

	"github.com/gin-gonic/gin"
)

// AuditHandler 暴露审计日志的只读查询接口，供管理员审计追溯。
type AuditHandler struct {
	auditSvc *audit.Service
}

// NewAuditHandler 创建审计查询 handler。
func NewAuditHandler(auditSvc *audit.Service) *AuditHandler {
	return &AuditHandler{auditSvc: auditSvc}
}

// List 分页查询审计日志。
// @Summary      查询审计日志
// @Description  管理员按动作/操作者/目标/时间范围过滤查询审计记录，仅可读
// @Tags         审计
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      object{action=string,actor_uuid=string,target_type=string,target_id=string,start=string,end=string,page=int,page_size=int}  true  "查询条件"
// @Success      200      {object}  pkg.Response
// @Router       /audit/list [post]
func (h *AuditHandler) List(c *gin.Context) {
	var req struct {
		Action     string `json:"action"`
		ActorUUID  string `json:"actor_uuid"`
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Start      string `json:"start"` // RFC3339
		End        string `json:"end"`   // RFC3339
		Page       int    `json:"page"`
		PageSize   int    `json:"page_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		pkg.Fail(c, pkg.INVALID_PARAMS, err.Error())
		return
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	q := audit.Query{
		Action:     req.Action,
		ActorUUID:  req.ActorUUID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Limit:      pageSize,
		Offset:     (page - 1) * pageSize,
	}
	if req.Start != "" {
		if t, err := time.Parse(time.RFC3339, req.Start); err == nil {
			q.Start = &t
		}
	}
	if req.End != "" {
		if t, err := time.Parse(time.RFC3339, req.End); err == nil {
			q.End = &t
		}
	}

	logs, total, err := h.auditSvc.List(q)
	if err != nil {
		pkg.HandleError(c, err)
		return
	}

	pkg.Success(c, gin.H{
		"list":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
