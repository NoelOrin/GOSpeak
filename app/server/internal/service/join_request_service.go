package service

import (
	"context"
	"errors"
	"log"
	"strings"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

var (
	// ErrJoinRequestNotFound 申请不存在或不属于该域。
	ErrJoinRequestNotFound = pkg.NewAppError(pkg.NOT_FOUND, "join request not found")
	// ErrJoinRequestAlreadyReviewed 申请已被他人处理（条件更新未命中）。
	ErrJoinRequestAlreadyReviewed = pkg.NewAppError(pkg.ALREADY_EXISTS, "join request has already been reviewed")
	// ErrJoinRequestNotPending 申请不是待审状态，无法撤销。
	ErrJoinRequestNotPending = pkg.NewAppError(pkg.INVALID_PARAMS, "join request is not pending")
)

// JoinRequestService 管理域加入申请的提交、撤销、审核与查询。
type JoinRequestService struct {
	db            *gorm.DB
	requestRepo   *repository.DomainJoinRequestRepository
	domainRepo    *repository.DomainRepository
	userRepo      *repository.UserRepository
	bus cacheBus
}

func NewJoinRequestService(
	db *gorm.DB,
	requestRepo *repository.DomainJoinRequestRepository,
	domainRepo *repository.DomainRepository,
	userRepo *repository.UserRepository,
) *JoinRequestService {
	return &JoinRequestService{db: db, requestRepo: requestRepo, domainRepo: domainRepo, userRepo: userRepo}
}

// SetEventBus 注入内部事件发布器：成员写入后广播失效事件（订阅方清缓存）。
func (s *JoinRequestService) SetEventBus(b cacheBus) {
	s.bus = b
}

// Submit 幂等提交申请。已是成员返回 ErrAlreadyMember；
// pending 直接返回原记录；rejected / approved（但已不是成员）回到 pending。
func (s *JoinRequestService) Submit(domainUUID, userUUID string, inviteID uint) (*model.DomainJoinRequest, error) {
	if domainUUID == "" || userUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid and user_uuid are required")
	}
	if _, err := s.domainRepo.GetByUUID(domainUUID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if _, err := s.domainRepo.GetMember(domainUUID, userUUID); err == nil {
		return nil, ErrAlreadyMember
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	req, err := s.requestRepo.Submit(domainUUID, userUUID, inviteID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if req.IsPending() {
		return req, nil
	}
	if err := s.requestRepo.Reopen(req.ID, inviteID); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return s.requestRepo.GetByDomainUser(domainUUID, userUUID)
}

// Find 返回用户在该域的申请；不存在返回 (nil, nil)。
func (s *JoinRequestService) Find(domainUUID, userUUID string) (*model.DomainJoinRequest, error) {
	req, err := s.requestRepo.GetByDomainUser(domainUUID, userUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return req, nil
}

// Cancel 撤销待审申请（物理删除）。
func (s *JoinRequestService) Cancel(domainUUID, userUUID string) error {
	req, err := s.Find(domainUUID, userUUID)
	if err != nil {
		return err
	}
	if req == nil {
		return ErrJoinRequestNotFound
	}
	if !req.IsPending() {
		return ErrJoinRequestNotPending
	}
	if err := s.requestRepo.DeleteByDomainUser(domainUUID, userUUID); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// Review 审核申请。approve=true 时在同一事务内写入 domain_members。
// 条件更新 RowsAffected=0 表示已被他人处理，返回 ErrJoinRequestAlreadyReviewed。
func (s *JoinRequestService) Review(domainUUID string, requestID uint, approve bool, reviewerUUID string) (*model.DomainJoinRequest, error) {
	if domainUUID == "" || requestID == 0 {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid and request_id are required")
	}
	req, err := s.requestRepo.GetByID(requestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJoinRequestNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if req.DomainUUID != domainUUID {
		return nil, ErrJoinRequestNotFound
	}
	if !req.IsPending() {
		return nil, ErrJoinRequestAlreadyReviewed
	}
	// 申请人已被删除时不允许通过：先落 rejected，再报错。
	if approve {
		if _, err := s.userRepo.GetByUUID(req.UserUUID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_, _ = s.requestRepo.Review(requestID, false, reviewerUUID)
				return nil, pkg.NewAppError(pkg.NOT_FOUND, "applicant no longer exists")
			}
			return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		repo := repository.NewDomainJoinRequestRepository(tx)
		ok, err := repo.Review(requestID, approve, reviewerUUID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrJoinRequestAlreadyReviewed
		}
		if !approve {
			return nil
		}
		domainRepo := repository.NewDomainRepository(tx)
		if _, err := domainRepo.GetMember(domainUUID, req.UserUUID); err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return domainRepo.AddMember(&model.DomainMember{
			DomainUUID: domainUUID,
			UserUUID:   req.UserUUID,
			RoleName:   model.DomainRoleMember,
		})
	})
	if txErr != nil {
		if appErr, ok := txErr.(*pkg.AppError); ok {
			return nil, appErr
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, txErr.Error())
	}
	if approve && s.bus != nil {
		payload := map[string]string{"domain_uuid": domainUUID, "user_uuid": req.UserUUID}
		if err := s.bus.PublishInternal(context.Background(), EventDomainMemberAdded, payload); err != nil {
			log.Printf("[JoinRequest] publish member added: %v", err)
		}
	}
	return s.requestRepo.GetByID(requestID)
}

// List 分页查询域内申请，可按状态过滤、按申请人名字搜索。
func (s *JoinRequestService) List(domainUUID, status, keyword string, page, pageSize int) ([]model.DomainJoinRequestInfo, int64, error) {
	if domainUUID == "" {
		return nil, 0, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	if status != "" && status != model.JoinRequestPending &&
		status != model.JoinRequestApproved && status != model.JoinRequestRejected {
		return nil, 0, pkg.NewAppError(pkg.INVALID_PARAMS, "status must be pending, approved or rejected")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := s.requestRepo.ListByDomain(domainUUID, status, strings.TrimSpace(keyword), page, pageSize)
	if err != nil {
		return nil, 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return items, total, nil
}

// Counts 返回域内各状态的申请数。
func (s *JoinRequestService) Counts(domainUUID string) (map[string]int64, error) {
	if domainUUID == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "domain_uuid is required")
	}
	counts, err := s.requestRepo.CountByStatus(domainUUID)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return counts, nil
}

// SetRequireApproval 读写域级「加入需审核」开关。
func (s *JoinRequestService) SetRequireApproval(domainUUID string, require bool) (*model.Domain, error) {
	domain, err := s.domainRepo.GetByUUID(domainUUID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDomainNotFound
		}
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	domain.RequireJoinApproval = require
	if err := s.domainRepo.Update(domain); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return domain, nil
}
