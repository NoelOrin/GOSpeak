package server

import (
	"GOSpeak/internal/logger"
	"GOSpeak/internal/repository"
	"GOSpeak/internal/service"

	"gorm.io/gorm"
)

// bootstrapControlPlane 执行启动写面初始化：种子、迁移回填、插件。
func bootstrapControlPlane(
	db *gorm.DB,
	roleRepo *repository.RoleRepository,
	userRepo *repository.UserRepository,
	permRepo *repository.PermissionRepository,
	sfuConfigSvc *service.SFUConfigService,
	permSvc *service.PermissionService,
) error {
	seedRoles(roleRepo)
	adminUUID := seedAdminUser(userRepo)
	if adminUUID != "" {
		if err := repository.EnsureDefaultDomain(db, adminUUID); err != nil {
			logger.WithComponent("Seed").Warnf("同步默认语音域失败: %v", err)
		}
	}
	if err := repository.BackfillDomainRoleDefaults(db); err != nil {
		logger.WithComponent("Seed").Warnf("存量域系统角色回填失败: %v", err)
	}
	seedPermissions(permRepo)
	// seed 之后 role_permissions 才有数据；启动早期的 UseCasbin 加载的是空表，
	// 必须重新构建 enforcer 才能把 seed 后的策略载入 Casbin，否则所有 RequirePermission 都会 403。
	if err := permSvc.UseCasbin(repository.NewCasbinAdapter(db)); err != nil {
		return err
	}
	if err := permSvc.LoadCache(); err != nil {
		return err
	}
	if err := sfuConfigSvc.SyncFromEnv(); err != nil {
		return err
	}
	return nil
}
