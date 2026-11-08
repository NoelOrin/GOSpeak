package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"gorm.io/gorm"
)

const (
	EmailSceneRegister      = "register"
	EmailSceneResetPassword = "reset_password"
	EmailSceneBindEmail     = "bind_email"
	EmailSceneChangeEmail   = "change_email"
)

type EmailVerificationService struct {
	repo          *repository.EmailVerificationCodeRepository
	userRepo      *repository.UserRepository
	emailSvc      *EmailService
	resolveConfig func() (*config.Config, error)
}

func NewEmailVerificationService(repo *repository.EmailVerificationCodeRepository, userRepo *repository.UserRepository, emailSvc *EmailService, resolveConfig func() (*config.Config, error)) *EmailVerificationService {
	return &EmailVerificationService{repo: repo, userRepo: userRepo, emailSvc: emailSvc, resolveConfig: resolveConfig}
}

func (s *EmailVerificationService) IsAvailable() bool {
	return s.emailSvc != nil && s.emailSvc.IsEnabled()
}

func (s *EmailVerificationService) SendCode(email, scene, ip string, userID *uint) (int, error) {
	if !s.IsAvailable() {
		return 0, pkg.NewAppError(pkg.EMAIL_NOT_CONFIGURED)
	}
	if !isSupportedEmailScene(scene) {
		return 0, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid email verification scene")
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return 0, pkg.NewAppError(pkg.INVALID_PARAMS, "email is required")
	}

	cfg, err := s.resolveConfig()
	if err != nil {
		return 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	ttl, err := time.ParseDuration(cfg.EmailCodeTTL)
	if err != nil || ttl <= 0 {
		ttl = 10 * time.Minute
	}
	cooldown, err := time.ParseDuration(cfg.EmailSendCooldown)
	if err != nil || cooldown <= 0 {
		cooldown = 60 * time.Second
	}

	latest, err := s.repo.GetLatestByEmailAndScene(email, scene)
	if err == nil && latest != nil && time.Since(latest.CreatedAt) < cooldown {
		return 0, pkg.NewAppError(pkg.EMAIL_SEND_TOO_FREQUENT)
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	dailyCount, err := s.repo.CountRecentByEmail(email, time.Now().Add(-24*time.Hour))
	if err != nil {
		return 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if dailyCount >= 10 {
		return 0, pkg.NewAppError(pkg.EMAIL_SEND_TOO_FREQUENT, "daily send limit reached")
	}

	if strings.TrimSpace(ip) != "" {
		ipCount, err := s.repo.CountRecentByIP(ip, time.Now().Add(-1*time.Hour))
		if err != nil {
			return 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if ipCount >= 20 {
			return 0, pkg.NewAppError(pkg.EMAIL_SEND_TOO_FREQUENT, "ip send limit reached")
		}
	}

	if scene == EmailSceneRegister {
		existing, err := s.userRepo.GetByEmail(email)
		if err == nil && existing != nil {
			return 0, pkg.NewAppError(pkg.EMAIL_ALREADY_EXISTS)
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
	}

	if scene == EmailSceneResetPassword {
		_, err := s.userRepo.GetByEmail(email)
		if err != nil && err != gorm.ErrRecordNotFound {
			return 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
		}
		if err == gorm.ErrRecordNotFound {
			return int(ttl.Seconds()), nil
		}
	}

	code := generateNumericCode(6)
	if err := s.repo.Create(&model.EmailVerificationCode{
		Email:        email,
		Scene:        scene,
		CodeHash:     s.hashCode(code, email, scene, cfg.EmailCodeSecret),
		UserID:       userID,
		IPAddress:    ip,
		ExpiresAt:    time.Now().Add(ttl),
		AttemptCount: 0,
	}); err != nil {
		return 0, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	if err := s.emailSvc.SendVerificationCode(email, scene, code); err != nil {
		return 0, err
	}
	return int(ttl.Seconds()), nil
}

func (s *EmailVerificationService) VerifyCode(email, scene, code string) error {
	if !s.IsAvailable() {
		return pkg.NewAppError(pkg.EMAIL_NOT_CONFIGURED)
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if !isSupportedEmailScene(scene) {
		return pkg.NewAppError(pkg.INVALID_PARAMS, "invalid email verification scene")
	}
	record, err := s.repo.GetLatestByEmailAndScene(email, scene)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkg.NewAppError(pkg.EMAIL_CODE_INVALID)
		}
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if record.UsedAt != nil {
		return pkg.NewAppError(pkg.EMAIL_CODE_ALREADY_USED)
	}
	if time.Now().After(record.ExpiresAt) {
		return pkg.NewAppError(pkg.EMAIL_CODE_EXPIRED)
	}
	if record.AttemptCount >= 5 {
		return pkg.NewAppError(pkg.EMAIL_CODE_MAX_ATTEMPTS)
	}

	cfg, err := s.resolveConfig()
	if err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	if record.CodeHash != s.hashCode(code, email, scene, cfg.EmailCodeSecret) {
		record.AttemptCount++
		_ = s.repo.Update(record)
		if record.AttemptCount >= 5 {
			return pkg.NewAppError(pkg.EMAIL_CODE_MAX_ATTEMPTS)
		}
		return pkg.NewAppError(pkg.EMAIL_CODE_INVALID)
	}
	now := time.Now()
	record.UsedAt = &now
	if err := s.repo.Update(record); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func (s *EmailVerificationService) VerifyRegisterEmail(email, code string) error {
	return s.VerifyCode(email, EmailSceneRegister, code)
}

func (s *EmailVerificationService) VerifyResetPasswordCode(email, code string) error {
	return s.VerifyCode(email, EmailSceneResetPassword, code)
}

func (s *EmailVerificationService) hashCode(code, email, scene, secret string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s", code, email, scene, secret)))
	return hex.EncodeToString(sum[:])
}

func isSupportedEmailScene(scene string) bool {
	switch scene {
	case EmailSceneRegister, EmailSceneResetPassword, EmailSceneBindEmail, EmailSceneChangeEmail:
		return true
	default:
		return false
	}
}

func generateNumericCode(length int) string {
	max := 1
	for range length {
		max *= 10
	}
	min := max / 10
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		return fmt.Sprintf("%0*d", length, 0)
	}
	value := int(n.Int64()) + min
	return fmt.Sprintf("%0*d", length, value)
}
