package service

import (
	"log"
	"strings"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

// InviteValidation 是邀请码校验成功后的结果。
type InviteValidation struct {
	Invite        *model.DomainInvite
	Domain        *model.Domain
	RemainingUses int
}

// InviteService 管理域邀请码的创建、校验、消耗与禁用。
type InviteService struct {
	db         *gorm.DB
	inviteRepo *repository.DomainInviteRepository
	domainRepo *repository.DomainRepository
	key        []byte
}

// NewInviteService 构造 InviteService。storageEncryptKey 为空时回退 jwtKey 并打 warn。
func NewInviteService(
	db *gorm.DB,
	inviteRepo *repository.DomainInviteRepository,
	domainRepo *repository.DomainRepository,
	storageEncryptKey, jwtKey string,
) *InviteService {
	if strings.TrimSpace(storageEncryptKey) == "" {
		log.Printf("[invite] STORAGE_ENCRYPT_KEY is empty; falling back to JWT_KEY for invite signing")
	}
	return &InviteService{
		db:         db,
		inviteRepo: inviteRepo,
		domainRepo: domainRepo,
		key:        DeriveInviteKey(storageEncryptKey, jwtKey),
	}
}

// Create 为域创建邀请码。expiresAt 为 nil 表示永不过期；maxUses 为 0 表示不限。
func (s *InviteService) Create(domainUUID, label string, expiresAt *time.Time, maxUses int, createdBy string) (*model.DomainInvite, error) {
	if domainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	if maxUses < 0 || maxUses > InviteMaxUsesLimit {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "max_uses must be between 0 and 65535")
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "expires_at must be in the future")
	}
	if _, err := s.domainRepo.GetByUUID(domainUUID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// codeID 由自增主键决定，因此先插入占位行拿到 ID，再回填签名码。
	invite := &model.DomainInvite{
		DomainUUID: domainUUID,
		Label:      strings.TrimSpace(label),
		ExpiresAt:  expiresAt,
		MaxUses:    maxUses,
		CreatedBy:  createdBy,
	}
	if err := s.inviteRepo.Create(invite); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	plain := InviteCode{
		DomainPrefix: DomainPrefixOf(domainUUID),
		InviteID:     uint32(invite.ID),
		MaxUses:      uint16(maxUses),
	}
	if expiresAt != nil {
		plain.ExpiresAt = *expiresAt
	}
	code, err := EncodeInviteCode(s.key, plain)
	if err != nil {
		return nil, err
	}
	invite.Code = code
	if err := s.inviteRepo.UpdateCode(invite.ID, code); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return invite, nil
}

// CreateDefaultForDomain 为新域生成默认邀请码（永不过期、不限次数）。
func (s *InviteService) CreateDefaultForDomain(domainUUID, createdBy string) (*model.DomainInvite, error) {
	return s.Create(domainUUID, "默认邀请码", nil, 0, createdBy)
}

// Validate 校验邀请码并返回其域。失败时统一返回 INVALID_PARAMS，
// 且不泄露任何域信息。
func (s *InviteService) Validate(raw string) (*InviteValidation, error) {
	trimmed := strings.TrimSpace(raw)
	code, err := DecodeInviteCode(s.key, trimmed, time.Now())
	if err != nil {
		return nil, err
	}
	invite, err := s.inviteRepo.GetByID(uint(code.InviteID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInviteCodeInvalid
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if invite.Code != trimmed {
		return nil, ErrInviteCodeInvalid
	}
	if invite.IsDisabled() {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is disabled")
	}
	if invite.IsExhausted() {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invite code has reached its usage limit")
	}
	domain, err := s.domainRepo.GetByUUID(invite.DomainUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInviteCodeInvalid
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	remaining := -1
	if invite.MaxUses > 0 {
		remaining = invite.MaxUses - invite.UsedCount
	}
	return &InviteValidation{Invite: invite, Domain: domain, RemainingUses: remaining}, nil
}

// Consume 在事务内消耗一次额度并写使用记录。recordSuccess 为 false 时只写失败记录。
func (s *InviteService) Consume(raw, userUUID string, recordSuccess bool) (*model.Domain, error) {
	trimmed := strings.TrimSpace(raw)
	code, decodeErr := DecodeInviteCode(s.key, trimmed, time.Now())
	if decodeErr != nil {
		return nil, decodeErr
	}
	invite, err := s.inviteRepo.GetByID(uint(code.InviteID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInviteCodeInvalid
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	// 条件更新本身即并发保护；RowsAffected=0 表示已禁用或次数耗尽。
	ok, err := s.inviteRepo.ConsumeUse(invite.ID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if !ok {
		reason := model.InviteFailExhausted
		if invite.IsDisabled() {
			reason = model.InviteFailDisabled
		}
		// 失败记录独立事务写入，避免随主流程回滚而丢失审计。
		_ = s.inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
			InviteID: invite.ID, DomainUUID: invite.DomainUUID, UserUUID: userUUID,
			Code: invite.Code, Success: false, FailReason: reason,
		})
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is no longer usable")
	}

	domain, err := s.domainRepo.GetByUUID(invite.DomainUUID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInviteCodeInvalid
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.inviteRepo.CreateRedemption(&model.DomainInviteRedemption{
		InviteID: invite.ID, DomainUUID: invite.DomainUUID, UserUUID: userUUID,
		Code: invite.Code, Success: recordSuccess,
	}); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return domain, nil
}

// Disable 禁用邀请码。
func (s *InviteService) Disable(inviteID uint, operatorUUID string) error {
	if _, err := s.inviteRepo.GetByID(inviteID); err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.NewAppError(pkg.NOT_FOUND, "invite code not found")
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if err := s.inviteRepo.Disable(inviteID, operatorUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// List 列出域内全部邀请码。
func (s *InviteService) List(domainUUID string) ([]model.DomainInvite, error) {
	list, err := s.inviteRepo.ListByDomain(domainUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, nil
}

// ListRedemptions 分页查询某邀请码的使用记录。
func (s *InviteService) ListRedemptions(inviteID uint, page, pageSize int) ([]model.DomainInviteRedemption, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	list, total, err := s.inviteRepo.ListRedemptions(inviteID, page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return list, total, nil
}
