package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const botNamePrefix = "bot_"

type CreateBotRequest struct {
	Name      string `json:"name" binding:"required"`
	Role      string `json:"role" binding:"required"`
	ExpiresIn string `json:"expires_in"`
}

type CreateBotResult struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

type BotService struct {
	userRepo *repository.UserRepository
	botRepo  *repository.BotTokenRepository
}

func NewBotService(userRepo *repository.UserRepository, botRepo *repository.BotTokenRepository) *BotService {
	return &BotService{userRepo: userRepo, botRepo: botRepo}
}

func (s *BotService) Create(req *CreateBotRequest) (*CreateBotResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "name is required")
	}
	if strings.HasPrefix(name, botNamePrefix) {
		return nil, pkg.NewAppError(pkg.INVALID_PARAMS, "name should not contain 'bot_' prefix")
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}

	botUsername := botNamePrefix + name
	existing, _ := s.userRepo.GetByName(botUsername)
	if existing != nil {
		return nil, pkg.NewAppError(pkg.USERNAME_EXISTS, "bot name already exists")
	}

	randomPwd := randomHex(32)
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	displayName := fmt.Sprintf("Bot-%s", name)
	user := &model.User{
		Name:        botUsername,
		DisplayName: displayName,
		Password:    string(hashedPwd),
		Role:        role,
		IsBot:       true,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	expiresAt, err := parseBotExpiry(req.ExpiresIn)
	if err != nil {
		_ = s.userRepo.Delete(user.ID)
		return nil, err
	}

	botToken := &model.BotToken{
		UUID:      uuid.New().String(),
		Name:      name,
		UserUUID:  user.UUID,
		Role:      role,
		ExpiresAt: expiresAt,
	}
	if err := s.botRepo.Create(botToken); err != nil {
		_ = s.userRepo.Delete(user.ID)
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	token, err := pkg.GenerateToken(user.Name, user.DisplayName, user.UUID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}

	return &CreateBotResult{Token: token, User: user}, nil
}

func (s *BotService) List() ([]model.BotToken, error) {
	return s.botRepo.List()
}

func (s *BotService) Revoke(uuid string) error {
	_, err := s.botRepo.GetByUUID(uuid)
	if err != nil {
		return pkg.NewAppError(pkg.NOT_FOUND, "bot token not found")
	}
	if err := s.botRepo.Revoke(uuid); err != nil {
		return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
	}
	return nil
}

func parseBotExpiry(expiresIn string) (time.Time, error) {
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

func randomHex(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
