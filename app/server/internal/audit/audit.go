// Package audit 提供管理类敏感操作的审计日志写入与查询能力。
// 写入为异步 best-effort：Log 仅把记录推入带缓冲队列后立即返回，后台 goroutine 落库；
// 队列满或被优雅关闭时丢弃并记日志，绝不阻塞主业务。查询为只读、同步。
package audit

import (
	"GOSpeak/internal/logger"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"github.com/gin-gonic/gin"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
)

const queueSize = 1024

const (
	AuditMaxLimit     = 200
	AuditDefaultLimit = 50
	auditMaxLimit     = AuditMaxLimit
	auditDefaultLimit = AuditDefaultLimit
)

// 审计动作常量
const (
	ActionKickMember  = "kick_member"
	ActionMuteUser    = "mute_user"
	ActionUnmuteUser  = "unmute_user"
	ActionDeleteRoom  = "delete_room"
	ActionDeleteUser  = "delete_user"
	ActionResetInvite = "reset_invite"
)

// 审计目标类型
const (
	TargetMember = "member"
	TargetRoom   = "room"
	TargetUser   = "user"
	TargetMute   = "mute"
	TargetDomain = "domain"
)

var (
	validActions = map[string]struct{}{
		ActionKickMember:  {},
		ActionMuteUser:    {},
		ActionUnmuteUser:  {},
		ActionDeleteRoom:  {},
		ActionDeleteUser:  {},
		ActionResetInvite: {},
	}
	validTargetTypes = map[string]struct{}{
		TargetMember: {},
		TargetRoom:   {},
		TargetUser:   {},
		TargetMute:   {},
		TargetDomain: {},
	}
)

// IsValidAction 校验 action 是否为白名单。
func IsValidAction(a string) bool {
	_, ok := validActions[a]
	return ok
}

// IsValidTargetType 校验 target_type 是否为白名单。
func IsValidTargetType(t string) bool {
	_, ok := validTargetTypes[t]
	return ok
}

// AuditIP 返回审计可信的客户端 IP，优先使用 RemoteIP（受 TrustedProxies 约束），避免 X-Forwarded-For 伪造。
func AuditIP(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if ip := c.RemoteIP(); ip != "" {
		return ip
	}
	return c.ClientIP()
}

// Entry 一条审计记录的结构化输入。
type Entry struct {
	ActorID    uint
	ActorUUID  string
	ActorName  string
	Action     string
	TargetType string
	TargetID   string
	Detail     string
	IP         string
	Success    bool
}

// Query 审计记录查询条件（均为可选过滤）。
type Query struct {
	Action     string
	ActorUUID  string
	TargetType string
	TargetID   string
	Start      *time.Time
	End        *time.Time
	Limit      int
	Offset     int
}

// Service 审计服务：异步写入 + 只读查询。
type Service struct {
	db      *gorm.DB
	queue   chan model.AuditLog
	mu      sync.Mutex
	closed  bool
	wg      sync.WaitGroup
	dropped atomic.Int64
}

// NewService 创建审计服务并启动后台落库 goroutine。
// db==nil 时不启动 worker，直接返回 nil，避免 nil.Create panic。
func NewService(db *gorm.DB) *Service {
	if db == nil {
		return nil
	}
	s := &Service{db: db, queue: make(chan model.AuditLog, queueSize)}
	s.wg.Add(1)
	go s.worker()
	return s
}

// worker 消费队列并落库，直到队列被 Stop 关闭。
func (s *Service) worker() {
	defer s.wg.Done()
	for rec := range s.queue {
		if s.db == nil {
			continue
		}
		if err := s.db.Create(&rec).Error; err != nil {
			logger.WithComponent("Audit").Errorf("[audit] write failed action=%s actor=%s target=%s err=%v", rec.Action, rec.ActorUUID, rec.TargetID, err)
		}
	}
}

// Log 将审计记录推入队列后立即返回（非阻塞）。队列满或已关闭时丢弃并记日志。
func (s *Service) Log(e Entry) {
	if s == nil || s.db == nil {
		return
	}
	rec := model.AuditLog{
		ActorID:    e.ActorID,
		ActorUUID:  e.ActorUUID,
		ActorName:  e.ActorName,
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		Detail:     e.Detail,
		IP:         e.IP,
		Success:    e.Success,
	}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return
	}
	select {
	case s.queue <- rec:
		return
	default:
		n := s.dropped.Add(1)
		logger.WithComponent("Audit").Warnf("[audit] queue full, dropped action=%s actor=%s target=%s dropped_total=%d", e.Action, e.ActorUUID, e.TargetID, n)
	}
}

// Dropped 返回队列满丢弃的总数。
func (s *Service) Dropped() int64 {
	if s == nil {
		return 0
	}
	return s.dropped.Load()
}

// List 分页查询审计日志，返回当前页记录与符合条件的总数。
func (s *Service) List(q Query) ([]model.AuditLog, int64, error) {
	if s == nil || s.db == nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, "audit service not initialized")
	}
	db := s.db.Model(&model.AuditLog{})
	if q.Action != "" {
		db = db.Where("action = ?", q.Action)
	}
	if q.ActorUUID != "" {
		db = db.Where("actor_uuid = ?", q.ActorUUID)
	}
	if q.TargetType != "" {
		db = db.Where("target_type = ?", q.TargetType)
	}
	if q.TargetID != "" {
		db = db.Where("target_id = ?", q.TargetID)
	}
	if q.Start != nil {
		db = db.Where("created_at >= ?", q.Start)
	}
	if q.End != nil {
		db = db.Where("created_at <= ?", q.End)
	}
	var total int64
	countDB := db.Session(&gorm.Session{})
	if err := countDB.Count(&total).Error; err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	limit := q.Limit
	if limit <= 0 || limit > auditMaxLimit {
		limit = auditDefaultLimit
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}
	var logs []model.AuditLog
	if err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return logs, total, nil
}

// Stop 关闭队列并等待已入队记录全部落库，应在进程优雅关闭时调用。
func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.queue)
	s.mu.Unlock()
	s.wg.Wait()
}
