package service

import (
	"errors"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// maxGuestNicknameLen 访客昵称最大长度（按字符计）。
const maxGuestNicknameLen = 24

// GuestJoinRequest 访客加入域请求：invite_code 与 domain_uuid 二选一；
// 走 domain_uuid 时目标域必须公开（IsPublic）。
type GuestJoinRequest struct {
	Nickname   string `json:"nickname" binding:"required,max=24"`
	InviteCode string `json:"invite_code"`
	DomainUUID string `json:"domain_uuid"`
}

// GuestJoinResponse 访客签发结果：双 Token + 访客用户 + 加入的域。
type GuestJoinResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	User         *model.User   `json:"user"`
	Domain       *model.Domain `json:"domain"`
}

// GuestService 访客访问服务：核发访客身份（users 行 + domain_members guest 角色）
// 与域内访客治理（封禁/解封/封禁列表）。onlineCount 为依赖注入的域内在线
// guest 统计函数（由装配层接入 WS Hub，测试注入桩）；传 nil 表示不做上限判断。
type GuestService struct {
	db          *gorm.DB
	userRepo    *repository.UserRepository
	domainRepo  *repository.DomainRepository
	banRepo     *repository.GuestBanRepo
	auth        *AuthService
	onlineCount func(domainUUID string) int
	// onlineGuestResolver 返回指定域内当前在线访客的机器名（users.name）列表。
	// 仅供 CleanupInactiveGuests 排除仍在线访客，避免误删挂机用户；nil 不排除。
	onlineGuestResolver func(domainUUID string) []string
}

// NewGuestService 构造 GuestService。
func NewGuestService(
	db *gorm.DB,
	userRepo *repository.UserRepository,
	domainRepo *repository.DomainRepository,
	banRepo *repository.GuestBanRepo,
	auth *AuthService,
	onlineCount func(domainUUID string) int,
) *GuestService {
	return &GuestService{db: db, userRepo: userRepo, domainRepo: domainRepo, banRepo: banRepo, auth: auth, onlineCount: onlineCount}
}

// SetOnlineGuestResolver 注入域内在线访客机器名解析（装配层接入 WS Hub）。
func (s *GuestService) SetOnlineGuestResolver(fn func(domainUUID string) []string) {
	s.onlineGuestResolver = fn
}

// Join 签发访客身份并加入 Domain。事务：users 行 + domain_members(role=guest)。
func (s *GuestService) Join(req *GuestJoinRequest) (*GuestJoinResponse, error) {
	nickname := req.Nickname
	if nickname == "" || len([]rune(nickname)) > maxGuestNicknameLen {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "nickname required and must be at most 24 characters")
	}
	if req.InviteCode == "" && req.DomainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invite_code or domain_uuid required")
	}
	domain, err := s.resolveDomain(req)
	if err != nil {
		return nil, err
	}
	if !domain.AllowGuest {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "guest access disabled")
	}
	if domain.GuestLimit > 0 && s.onlineCount != nil && s.onlineCount(domain.UUID) >= domain.GuestLimit {
		return nil, pkg.NewAppError(pkg.RATE_LIMITED, "guest limit reached")
	}
	user := &model.User{
		Name:        "guest_" + uuid.NewString()[:12],
		DisplayName: nickname,
		IsGuest:     true,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		member := &model.DomainMember{
			DomainUUID: domain.UUID,
			UserUUID:   user.UUID,
			Nickname:   nickname,
			RoleName:   model.DomainRoleGuest,
		}
		return tx.Create(member).Error
	})
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	tokens, err := s.auth.issueTokens(user)
	if err != nil {
		return nil, err
	}
	// 邀请码仅管理员可见，不随访客签发响应下发。
	sanitized := *domain
	sanitized.InviteCode = ""
	return &GuestJoinResponse{AccessToken: tokens.Access, RefreshToken: tokens.Refresh, User: user, Domain: &sanitized}, nil
}

// resolveDomain 有 invite_code 按 code 查；否则按 domain_uuid 查且要求域公开。
func (s *GuestService) resolveDomain(req *GuestJoinRequest) (*model.Domain, error) {
	if req.InviteCode != "" {
		domain, err := s.domainRepo.GetByInviteCode(req.InviteCode)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, pkg.NewAppError(pkg.NOT_FOUND, "domain not found")
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return domain, nil
	}
	domain, err := s.domainRepo.GetByUUID(req.DomainUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.NOT_FOUND, "domain not found")
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !domain.IsPublic {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain is not public; invite_code required")
	}
	return domain, nil
}

// isGuestMember 校验目标用户是该域的 guest 成员（domain_members 命中且 user.IsGuest）。
func (s *GuestService) isGuestMember(domainUUID, userUUID string) (bool, error) {
	member, err := s.domainRepo.GetMember(domainUUID, userUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if member.RoleName != model.DomainRoleGuest {
		return false, nil
	}
	user, err := s.userRepo.GetByUUID(userUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return user.IsGuest, nil
}

// Ban 封禁域内访客。幂等：已存在活跃封禁则更新 reason/expires_at；
// durationHours<=0 表示永久封禁。踢下线由 handler 层调用现有 kick 路径完成。
func (s *GuestService) Ban(domainUUID, operatorUUID, guestUUID, reason string, durationHours int) error {
	ok, err := s.isGuestMember(domainUUID, guestUUID)
	if err != nil {
		return err
	}
	if !ok {
		return pkg.NewAppError(pkg.NOT_FOUND, "guest member not found")
	}
	var expiresAt *time.Time
	if durationHours > 0 {
		t := time.Now().Add(time.Duration(durationHours) * time.Hour)
		expiresAt = &t
	}
	// 注意：对已存在的临时封禁再次 Ban 会以其 durationHours 重算；传 0 会把临时封禁覆盖为永久。
	if existing, exErr := s.banRepo.FindActive(domainUUID, guestUUID); exErr != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, exErr.Error())
	} else if existing != nil {
		existing.Reason = reason
		existing.BannedBy = operatorUUID
		existing.ExpiresAt = expiresAt
		if err := s.db.Save(existing).Error; err != nil {
			return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		return nil
	}
	ban := &model.DomainGuestBan{
		DomainUUID: domainUUID,
		UserUUID:   guestUUID,
		Reason:     reason,
		BannedBy:   operatorUUID,
		ExpiresAt:  expiresAt,
	}
	if err := s.banRepo.Create(ban); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// Unban 解除域内访客封禁（物理删除封禁记录）。
func (s *GuestService) Unban(domainUUID, guestUUID string) error {
	if err := s.banRepo.Delete(domainUUID, guestUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// ListBans 列出域内全部封禁记录（含已过期），按时间倒序。
func (s *GuestService) ListBans(domainUUID string) ([]model.DomainGuestBan, error) {
	list, err := s.banRepo.ListByDomain(domainUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, nil
}

// IsGuest 判断用户是否访客（供 guest 守卫中间件适配）。
func (s *GuestService) IsGuest(userUUID string) bool {
	user, err := s.userRepo.GetByUUID(userUUID)
	if err != nil || user == nil {
		return false
	}
	return user.IsGuest
}

// IsGuestBanned 判断用户在指定域是否有活跃封禁；数据库异常直接上抛。
func (s *GuestService) IsGuestBanned(domainUUID, userUUID string) (bool, error) {
	ban, err := s.banRepo.FindActive(domainUUID, userUUID)
	if err != nil {
		return false, err
	}
	return ban != nil, nil
}

// IsGuestDomainMember 校验用户是否为该域 guest 成员；数据库异常直接上抛。
func (s *GuestService) IsGuestDomainMember(domainUUID, userUUID string) (bool, error) {
	return s.isGuestMember(domainUUID, userUUID)
}

// GuestConfigUpdate 局部更新域访客配置；nil 字段不修改。
type GuestConfigUpdate struct {
	AllowGuest      *bool
	GuestCanListen  *bool
	GuestCanSpeak   *bool
	GuestCanMessage *bool
	GuestLimit      *int
}

// GetConfig 读取域访客配置（含能力开关与上限）。
func (s *GuestService) GetConfig(domainUUID string) (*model.Domain, error) {
	domain, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.NOT_FOUND, "domain not found")
	}
	return domain, nil
}

// UpdateConfig 更新域访客配置。
func (s *GuestService) UpdateConfig(domainUUID string, upd GuestConfigUpdate) (*model.Domain, error) {
	domain, err := s.GetConfig(domainUUID)
	if err != nil {
		return nil, err
	}
	if upd.AllowGuest != nil {
		domain.AllowGuest = *upd.AllowGuest
	}
	if upd.GuestCanListen != nil {
		domain.GuestCanListen = *upd.GuestCanListen
	}
	if upd.GuestCanSpeak != nil {
		domain.GuestCanSpeak = *upd.GuestCanSpeak
	}
	if upd.GuestCanMessage != nil {
		domain.GuestCanMessage = *upd.GuestCanMessage
	}
	if upd.GuestLimit != nil {
		if *upd.GuestLimit < 0 {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "guest_limit must be >= 0")
		}
		domain.GuestLimit = *upd.GuestLimit
	}
	if err := s.domainRepo.Update(domain); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return domain, nil
}

// Renew 已有访客身份加入另一个 Domain：复用 user 行，仅新增成员关系。
// 已是成员时幂等返回。封禁/开关/上限检查与 Join 一致（额外校验封禁，
// 因为身份已存在，可能已被目标域拉黑）。
func (s *GuestService) Renew(userUUID string, req *GuestJoinRequest) (*GuestJoinResponse, error) {
	user, err := s.userRepo.GetByUUID(userUUID)
	if err != nil || user == nil || !user.IsGuest {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "not a guest identity")
	}
	if req.InviteCode == "" && req.DomainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invite_code or domain_uuid required")
	}
	domain, err := s.resolveDomain(req)
	if err != nil {
		return nil, err
	}
	if !domain.AllowGuest {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "guest access disabled")
	}
	if banned, err := s.banRepo.FindActive(domain.UUID, user.UUID); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	} else if banned != nil {
		return nil, pkg.NewAppError(pkg.FORBIDDEN, "guest has been banned")
	}
	if domain.GuestLimit > 0 && s.onlineCount != nil && s.onlineCount(domain.UUID) >= domain.GuestLimit {
		return nil, pkg.NewAppError(pkg.RATE_LIMITED, "guest limit reached")
	}
	member := &model.DomainMember{
		DomainUUID: domain.UUID,
		UserUUID:   user.UUID,
		Nickname:   user.DisplayName,
		RoleName:   model.DomainRoleGuest,
	}
	if existing, getErr := s.domainRepo.GetMember(domain.UUID, user.UUID); getErr == nil && existing != nil {
		// 幂等：已是成员直接返回。防御校验角色必须是 guest，避免身份不一致被复用为进域凭证。
		if existing.RoleName != model.DomainRoleGuest {
			return nil, pkg.NewAppError(pkg.FORBIDDEN, "guest identity already bound to a non-guest membership")
		}
	} else if errors.Is(getErr, gorm.ErrRecordNotFound) {
		if err := s.db.Create(member).Error; err != nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	} else if getErr != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, getErr.Error())
	}
	tokens, err := s.auth.issueTokens(user)
	if err != nil {
		return nil, err
	}
	sanitized := *domain
	sanitized.InviteCode = ""
	return &GuestJoinResponse{AccessToken: tokens.Access, RefreshToken: tokens.Refresh, User: user, Domain: &sanitized}, nil
}

// CleanupInactiveGuests 删除指定域内 updated_at 早于 cutoff 的访客 users 行及其
// 该域成员关系；消息保留，author_uuid 悬空由前端显示为已注销访客。
// 仅处理 domainUUID 域内的 guest 成员，永不影响其它域；仍在线访客会排除。
func (s *GuestService) CleanupInactiveGuests(domainUUID string, days int) (int64, error) {
	if days <= 0 {
		return 0, pkg.NewAppError(pkg.INVALID_PARAMS, "days must be positive")
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	var count int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 候选：目标域内的 guest 成员。先用 Pluck 物化，避免后续删除改动
		// 同一张读表导致惰性子查询游标漂移（候选被清空）。
		var candidates []string
		if err := tx.Model(&model.DomainMember{}).
			Where("domain_uuid = ?", domainUUID).
			Where("role_name = ?", model.DomainRoleGuest).
			Pluck("user_uuid", &candidates).Error; err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}
		// 仅清本域成员关系，不影响其它域。
		if err := tx.Where("domain_uuid = ? AND user_uuid IN ?", domainUUID, candidates).
			Delete(&model.DomainMember{}).Error; err != nil {
			return err
		}
		// user 行仅在“该 guest 已不属于任何域成员”时才删除，避免跨域误删。
		q := tx.Where("is_guest = ? AND updated_at < ? AND uuid IN ?", true, cutoff, candidates).
			Where("NOT EXISTS (SELECT 1 FROM domain_members dm WHERE dm.user_uuid = users.uuid)")
		if s.onlineGuestResolver != nil {
			if online := s.onlineGuestResolver(domainUUID); len(online) > 0 {
				q = q.Where("name NOT IN ?", online)
			}
		}
		res := q.Delete(&model.User{})
		count = res.RowsAffected
		return res.Error
	})
	if err != nil {
		return 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return count, nil
}

// GuestCaps 读取域访客三项能力开关（听/说/发消息）。
// 域不存在时全 false，保证 fail-closed。
func (s *GuestService) GuestCaps(domainUUID string) (listen, speak, message bool) {
	domain, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil || domain == nil {
		return false, false, false
	}
	return domain.GuestCanListen, domain.GuestCanSpeak, domain.GuestCanMessage
}

// GuestLimitReached 判断域内在线访客数是否已达上限。
// onlineIdentities 为该域全部在线身份名；软上限语义，超限返回 true。
func (s *GuestService) GuestLimitReached(domainUUID string, onlineIdentities []string) bool {
	domain, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil || domain == nil {
		return true
	}
	if domain.GuestLimit <= 0 {
		return false
	}
	count := 0
	for _, identity := range onlineIdentities {
		if model.IsGuestName(identity) {
			count++
		}
	}
	return count >= domain.GuestLimit
}

// SetOnlineCounter 装配层注入域内在线访客统计（WS Hub），Join 签发上限判断使用。
func (s *GuestService) SetOnlineCounter(fn func(domainUUID string) int) {
	s.onlineCount = fn
}
