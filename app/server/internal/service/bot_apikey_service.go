package service

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const botKeyPrefix = "gk_"

type BotAPIKeyService struct {
	repo *repository.BotAPIKeyRepository
}

func NewBotAPIKeyService(repo *repository.BotAPIKeyRepository) *BotAPIKeyService {
	return &BotAPIKeyService{repo: repo}
}

// CreateRequest 创建 Bot Key 的请求参数。
type CreateBotKeyRequest struct {
	Name        string   `json:"name" binding:"required"`
	Permissions []string `json:"permissions"`
	ExpiresIn   string   `json:"expires_in"` // 如 "720h"、"30d"，为空表示不过期
}

// CreateResult 创建结果，PlainKey 仅在创建时返回一次。
type CreateBotKeyResult struct {
	Key      model.BotAPIKeyResponse
	PlainKey string
}

func (s *BotAPIKeyService) Create(req CreateBotKeyRequest, createdBy string) (*CreateBotKeyResult, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "name is required")
	}

	perms := normalizePerms(req.Permissions)
	if err := s.validatePerms(perms); err != nil {
		return nil, err
	}

	expiresAt, err := parseExpiry(req.ExpiresIn)
	if err != nil {
		return nil, err
	}

	plain := generatePlainKey()
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	key := &model.BotAPIKey{
		UUID:        uuid.New().String(),
		Name:        req.Name,
		KeyHash:     string(hash),
		Permissions: strings.Join(perms, ","),
		CreatedBy:   createdBy,
		ExpiresAt:   expiresAt,
	}
	if err := s.repo.Create(key); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return &CreateBotKeyResult{
		Key:      toResponse(key),
		PlainKey: plain,
	}, nil
}

func (s *BotAPIKeyService) List(createdBy string) ([]model.BotAPIKeyResponse, error) {
	keys, err := s.repo.List(createdBy)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	out := make([]model.BotAPIKeyResponse, 0, len(keys))
	for i := range keys {
		out = append(out, toResponse(&keys[i]))
	}
	return out, nil
}

func (s *BotAPIKeyService) Revoke(uuid string, createdBy string) error {
	key, err := s.repo.GetByUUID(uuid)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "bot api key not found")
	}
	if createdBy != "" && key.CreatedBy != createdBy {
		return pkg.NewAppError(pkg.FORBIDDEN, "not the owner of this key")
	}
	if err := s.repo.Revoke(uuid); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

// Resolve 按明文 key 解析出有效（未吊销、未过期）的 Bot 权限集合。
// bcrypt 哈希不可逆且每次不同，故遍历所有候选 key 用 CompareHashAndPassword 比对。
// 返回 nil 表示 key 无效；调用方应据此回退到普通 JWT 校验。
func (s *BotAPIKeyService) Resolve(plain string) ([]string, bool) {
	candidates, err := s.repo.ListActive()
	if err != nil {
		return nil, false
	}
	for i := range candidates {
		key := &candidates[i]
		if bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(plain)) != nil {
			continue
		}
		if key.Revoked || time.Now().After(key.ExpiresAt) {
			return nil, false
		}
		_ = s.repo.TouchLastUsed(key.UUID, time.Now())
		if key.Permissions == "" {
			return []string{}, true
		}
		return strings.Split(key.Permissions, ","), true
	}
	return nil, false
}

func (s *BotAPIKeyService) validatePerms(perms []string) error {
	for _, p := range perms {
		if model.IsAdminOnlyPermission(p) {
			return pkg.NewAppError(pkg.FORBIDDEN, "permission "+p+" is admin-only and cannot be granted to a bot")
		}
	}
	return nil
}

func normalizePerms(perms []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func parseExpiry(expiresIn string) (time.Time, error) {
	expiresIn = strings.TrimSpace(expiresIn)
	if expiresIn == "" {
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), nil
	}
	d, err := time.ParseDuration(expiresIn)
	if err != nil {
		return time.Time{}, pkg.NewAppError(pkg.INVALID_PARAMS, "invalid expires_in, use Go duration like 720h or 30d")
	}
	if d <= 0 {
		return time.Time{}, pkg.NewAppError(pkg.INVALID_PARAMS, "expires_in must be positive")
	}
	return time.Now().Add(d), nil
}

func generatePlainKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return botKeyPrefix + hex.EncodeToString(b)
}

func toResponse(k *model.BotAPIKey) model.BotAPIKeyResponse {
	var perms []string
	if k.Permissions != "" {
		perms = strings.Split(k.Permissions, ",")
	} else {
		perms = []string{}
	}
	return model.BotAPIKeyResponse{
		ID:          k.ID,
		UUID:        k.UUID,
		Name:        k.Name,
		Permissions: perms,
		CreatedBy:   k.CreatedBy,
		ExpiresAt:   k.ExpiresAt,
		LastUsedAt:  k.LastUsedAt,
		Revoked:     k.Revoked,
		CreatedAt:   k.CreatedAt,
	}
}
