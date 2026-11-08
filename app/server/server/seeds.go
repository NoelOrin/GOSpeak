package server

import (
	"GOSpeak/internal/config"
	"GOSpeak/internal/logger"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func seedPermissions(permRepo *repository.PermissionRepository) {
	// 种子权限定义
	for i := range model.DefaultPermissions {
		if err := permRepo.CreateIfNotExists(&model.DefaultPermissions[i]); err != nil {
			logger.WithComponent("Seed").Warnf("创建权限 %s 失败: %v", model.DefaultPermissions[i].Code, err)
		}
	}

	// 种子角色-权限映射
	for roleName, codes := range model.DefaultRolePermissions {
		if err := permRepo.EnsureRolePermissions(roleName, codes); err != nil {
			logger.WithComponent("Seed").Warnf("同步角色 %s 权限失败: %v", roleName, err)
		}
	}
	logger.WithComponent("Seed").Info("权限系统初始化完成")
}

func seedRoles(roleRepo *repository.RoleRepository) {
	for i := range model.DefaultRoles {
		if err := roleRepo.CreateIfNotExists(&model.DefaultRoles[i]); err != nil {
			logger.WithComponent("Seed").Warnf("创建角色 %s 失败: %v", model.DefaultRoles[i].Name, err)
		}
	}
	roles, err := roleRepo.List()
	if err != nil {
		logger.WithComponent("Seed").Errorf("加载角色列表失败: %v", err)
		return
	}
	model.LoadRoleCache(roles)
	logger.WithComponent("Seed").Infof("已加载 %d 个角色", len(roles))
}

func loadRoles(roleRepo *repository.RoleRepository) {
	roles, err := roleRepo.List()
	if err != nil {
		logger.WithComponent("Cluster").Warnf("load roles failed: %v", err)
		return
	}
	model.LoadRoleCache(roles)
	logger.WithComponent("Cluster").Debugf("loaded %d roles", len(roles))
}

func wsAllowedOrigins(cfg *config.Config) []string {
	if origins := cfg.WSAllowedOriginsList(); len(origins) > 0 {
		return origins
	}
	return []string{cfg.CORSOrigin}
}

func seedAdminUser(userRepo *repository.UserRepository) string {
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(service.DefaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		logger.WithComponent("Seed").Errorf("生成密码哈希失败: %v", err)
		return ""
	}

	existing, _ := userRepo.GetByName("admin")
	if existing != nil {
		return existing.UUID
	}

	admin := &model.User{
		Name:        "admin",
		DisplayName: "管理员",
		Password:    string(hashedPwd),
		Role:        "admin",
	}
	if err := userRepo.Create(admin); err != nil {
		logger.WithComponent("Seed").Errorf("创建管理员用户失败: %v", err)
		return ""
	}
	logger.WithComponent("Seed").Infof("已创建管理员用户: admin / %s", service.DefaultAdminPassword)
	return admin.UUID
}

func init() {
	// 日志完整初始化在 StartGin 的 config 加载之后；此处仅做最小兜底
	gin.DisableConsoleColor()
}
