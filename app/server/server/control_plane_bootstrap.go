package server

import (
	"context"

	"GOSpeak/internal/logger"
	"GOSpeak/internal/plugin"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"gorm.io/gorm"
)

// bootstrapAgentControlPlane 只在抢到 Agent 主锁后执行写面初始化，
// 避免两个 Agent 同时启动时 seed/迁移/插件写入互相覆盖。
func bootstrapAgentControlPlane(
	db *gorm.DB,
	roleRepo *repository.RoleRepository,
	userRepo *repository.UserRepository,
	permRepo *repository.PermissionRepository,
	sfuConfigSvc *service.SFUConfigService,
	pluginReg *plugin.Registry,
	permSvc *service.PermissionService,
) error {
	seedRoles(roleRepo)
	adminUUID := seedAdminUser(userRepo)
	if adminUUID != "" {
		if err := repository.EnsureDefaultDomain(db, adminUUID); err != nil {
			logger.WithComponent("Seed").Warnf("同步默认语音域失败: %v", err)
		}
	}
	seedPermissions(permRepo)
	if err := permSvc.LoadCache(); err != nil {
		return err
	}
	if err := sfuConfigSvc.SyncFromEnv(); err != nil {
		return err
	}
	if err := pluginReg.InitAll(); err != nil {
		return err
	}
	if err := pluginReg.StartEnabled(context.Background()); err != nil {
		logger.WithComponent("Plugin").Warnf("start plugins: %v", err)
	}
	return nil
}
