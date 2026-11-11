package service

import (
	"errors"
	"strings"
	"unicode/utf8"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// 加入结果状态：joined 直接成为成员，pending 已提交待审申请。
const (
	JoinStatusJoined  = "joined"
	JoinStatusPending = "pending"
)

// maxJoinNicknameLen 域内昵称最大长度（按字符计）。
const maxJoinNicknameLen = 32

// InviteRegisterRequest 内联注册请求：注册 + 入域/提交申请。
type InviteRegisterRequest struct {
	Code      string `json:"code" binding:"required"`
	Username  string `json:"username" binding:"required"`
	Password  string `json:"password" binding:"required"`
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	EmailCode string `json:"email_code"`
}

// InviteJoinRequest 已登录用户加入域：code 与 domain_uuid 二选一。
type InviteJoinRequest struct {
	Code       string `json:"code"`
	DomainUUID string `json:"domain_uuid"`
}

// InviteJoinResponse 注册/加入的统一返回。
type InviteJoinResponse struct {
	Token        string        `json:"access_token,omitempty"`
	RefreshToken string        `json:"refresh_token,omitempty"`
	ExpiresIn    int64         `json:"expires_in,omitempty"`
	User         *model.User   `json:"user,omitempty"`
	Domain       *model.Domain `json:"domain"`
	Status       string        `json:"status"`
}

// JoinRequestView 是 /invite/my-request 的返回：申请本体 + 是否已是成员。
type JoinRequestView struct {
	Request  *model.DomainJoinRequest `json:"request"`
	Status   string                   `json:"status"`
	IsMember bool                     `json:"is_member"`
}

// InviteJoinService 负责内联注册、已登录加入、申请查询与撤销。
// 不改动第一部分 InviteService 的构造函数，仅复用其校验能力。
type InviteJoinService struct {
	db          *gorm.DB
	inviteSvc   *InviteService
	joinReqSvc  *JoinRequestService
	inviteRepo  *repository.DomainInviteRepository
	domainRepo  *repository.DomainRepository
	userRepo    *repository.UserRepository
	emailConfig *EmailConfigService
	emailVerify *EmailVerificationService
}

func NewInviteJoinService(
	db *gorm.DB,
	inviteSvc *InviteService,
	joinReqSvc *JoinRequestService,
	inviteRepo *repository.DomainInviteRepository,
	domainRepo *repository.DomainRepository,
	userRepo *repository.UserRepository,
	emailConfig *EmailConfigService,
	emailVerify *EmailVerificationService,
) *InviteJoinService {
	return &InviteJoinService{
		db: db, inviteSvc: inviteSvc, joinReqSvc: joinReqSvc,
		inviteRepo: inviteRepo, domainRepo: domainRepo, userRepo: userRepo,
		emailConfig: emailConfig, emailVerify: emailVerify,
	}
}

// RegisterAndJoin 内联注册并按域策略入域或提交申请。
// 校验失败时（码无效/过期/耗尽、用户名重复、密码太短、邮箱验证失败）
// 不写任何 users 行：所有写操作都在最后一次事务里。
func (s *InviteJoinService) RegisterAndJoin(req *InviteRegisterRequest) (*InviteJoinResponse, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "username is required")
	}
	if strings.HasPrefix(strings.ToLower(username), "guest_") {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "username cannot start with guest_")
	}
	if strings.HasPrefix(strings.ToLower(username), "bot_") {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS, "username prefix 'bot_' is reserved")
	}
	nickname := strings.TrimSpace(req.Nickname)
	if utf8.RuneCountInString(nickname) > maxJoinNicknameLen {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "nickname is too long")
	}

	validation, err := s.inviteSvc.Validate(req.Code)
	if err != nil {
		return nil, err
	}
	domain := validation.Domain

	existing, _ := s.userRepo.GetByName(username)
	if existing != nil {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
	}
	if s.emailConfig != nil && s.emailConfig.IsVerificationAvailable() {
		if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.EmailCode) == "" {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "email and email_code are required")
		}
		byEmail, err := s.userRepo.GetByEmail(req.Email)
		if err == nil && byEmail != nil {
			return nil, pkg.NewAppError(pkg.EMAIL_ALREADY_EXISTS)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if s.emailVerify == nil {
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, "email verification service is not initialized")
		}
		if err := s.emailVerify.VerifyRegisterEmail(req.Email, req.EmailCode); err != nil {
			return nil, err
		}
	}
	if err := validateNewPassword(req.Password); err != nil {
		return nil, err
	}
	hashed, err := authbossHasher.GenerateHash(req.Password)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	emailVerified := req.Email != "" && s.emailConfig != nil && s.emailConfig.IsVerificationAvailable()
	user := &model.User{
		Name:          username,
		Password:      hashed,
		Role:          "user",
		Email:         strings.TrimSpace(req.Email),
		EmailVerified: emailVerified,
	}

	status := JoinStatusJoined
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				if strings.Contains(strings.ToLower(err.Error()), "email") {
					return pkg.NewAppError(pkg.EMAIL_ALREADY_EXISTS)
				}
				return pkg.NewAppError(pkg.USERNAME_EXISTS)
			}
			return err
		}
		inviteRepo := repository.NewDomainInviteRepository(tx)
		ok, err := inviteRepo.ConsumeUse(validation.Invite.ID)
		if err != nil {
			return err
		}
		if !ok {
			reason := model.InviteFailExhausted
			if validation.Invite.IsDisabled() {
				reason = model.InviteFailDisabled
			}
			_ = inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
				InviteID: validation.Invite.ID, DomainUUID: domain.UUID, UserUUID: user.UUID,
				Code: validation.Invite.Code, Success: false, FailReason: reason,
			})
			return pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is no longer usable")
		}
		if err := inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
			InviteID: validation.Invite.ID, DomainUUID: domain.UUID, UserUUID: user.UUID,
			Code: validation.Invite.Code, Success: true,
		}); err != nil {
			return err
		}
		if domain.RequireJoinApproval {
			status = JoinStatusPending
			_, err := repository.NewDomainJoinRequestRepository(tx).Submit(domain.UUID, user.UUID, validation.Invite.ID)
			return err
		}
		return repository.NewDomainRepository(tx).AddMember(&model.DomainMember{
			DomainUUID: domain.UUID, UserUUID: user.UUID,
			Nickname: nickname, RoleName: model.DomainRoleMember,
		})
	})
	if txErr != nil {
		if appErr, ok := txErr.(*pkg.AppError); ok {
			return nil, appErr
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, txErr.Error())
	}

	access, refresh, err := generateTokenPair(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, err
	}
	return &InviteJoinResponse{
		Token: access, RefreshToken: refresh,
		ExpiresIn: pkg.AccessTokenExpiresIn(),
		User:      user, Domain: domain, Status: status,
	}, nil
}

// Join 已登录用户加入域。code 优先；只有 domain_uuid 时走公开域免码申请。
func (s *InviteJoinService) Join(userUUID string, req *InviteJoinRequest) (*InviteJoinResponse, error) {
	if userUUID == "" {
		return nil, pkg.NewAppError(pkg.TOKEN_NOT_EXIST, "user not authenticated")
	}
	code := strings.TrimSpace(req.Code)
	domainUUID := strings.TrimSpace(req.DomainUUID)
	if code == "" && domainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "code or domain_uuid is required")
	}

	var domain *model.Domain
	inviteID := uint(0)
	if code != "" {
		validation, err := s.inviteSvc.Validate(code)
		if err != nil {
			return nil, err
		}
		domain = validation.Domain
		inviteID = validation.Invite.ID
	} else {
		d, err := s.domainRepo.GetByUUID(domainUUID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrDomainNotFound
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if !d.IsPublic {
			return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "this domain requires an invite code")
		}
		domain = d
	}

	// 已是成员：幂等返回 joined，且不消耗邀请码额度。
	if _, err := s.domainRepo.GetMember(domain.UUID, userUUID); err == nil {
		return &InviteJoinResponse{Domain: domain, Status: JoinStatusJoined}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if domain.RequireJoinApproval {
		if _, err := s.joinReqSvc.Submit(domain.UUID, userUUID, inviteID); err != nil {
			return nil, err
		}
		return &InviteJoinResponse{Domain: domain, Status: JoinStatusPending}, nil
	}

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		if inviteID != 0 {
			inviteRepo := repository.NewDomainInviteRepository(tx)
			ok, err := inviteRepo.ConsumeUse(inviteID)
			if err != nil {
				return err
			}
			if !ok {
				_ = inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
					InviteID: inviteID, DomainUUID: domain.UUID, UserUUID: userUUID,
					Code: code, Success: false, FailReason: model.InviteFailExhausted,
				})
				return pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is no longer usable")
			}
			if err := inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
				InviteID: inviteID, DomainUUID: domain.UUID, UserUUID: userUUID,
				Code: code, Success: true,
			}); err != nil {
				return err
			}
		}
		return repository.NewDomainRepository(tx).AddMember(&model.DomainMember{
			DomainUUID: domain.UUID, UserUUID: userUUID, RoleName: model.DomainRoleMember,
		})
	})
	if txErr != nil {
		if appErr, ok := txErr.(*pkg.AppError); ok {
			return nil, appErr
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, txErr.Error())
	}
	return &InviteJoinResponse{Domain: domain, Status: JoinStatusJoined}, nil
}

// MyRequest 返回用户在某域的申请状态；无申请时 Status 为空串。
func (s *InviteJoinService) MyRequest(userUUID, domainUUID string) (*JoinRequestView, error) {
	if userUUID == "" || domainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	view := &JoinRequestView{}
	if _, err := s.domainRepo.GetMember(domainUUID, userUUID); err == nil {
		view.IsMember = true
	}
	req, err := s.joinReqSvc.Find(domainUUID, userUUID)
	if err != nil {
		return nil, err
	}
	if req != nil {
		view.Request = req
		view.Status = req.Status
	}
	return view, nil
}

// CancelRequest 撤销自己的待审申请。
func (s *InviteJoinService) CancelRequest(userUUID, domainUUID string) error {
	if userUUID == "" || domainUUID == "" {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	return s.joinReqSvc.Cancel(domainUUID, userUUID)
}
