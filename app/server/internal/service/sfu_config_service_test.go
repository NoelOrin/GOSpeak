package service

import (
	"errors"
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"GOSpeak/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newSFUConfigTestRepo(t *testing.T) *repository.SFUConfigRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.SFUConfig{}, &model.SFUActiveProvider{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repository.NewSFUConfigRepository(db)
}

func TestSyncFromEnv_PreservesExistingConfig(t *testing.T) {
	repo := newSFUConfigTestRepo(t)

	// 预置一条 LiveKit 配置（模拟用户已保存）
	if err := repo.Save(&model.SFUConfig{
		Provider:    "livekit",
		LiveKitHost: "user-saved-host",
	}); err != nil {
		t.Fatalf("seed livekit row: %v", err)
	}

	baseCfg := &config.Config{
		SFUProvider: "livekit",
		LiveKitHost: "env-default-host",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync from env: %v", err)
	}

	// 已存在则不应被 env 覆盖
	lkCfg, err := repo.GetByProvider("livekit")
	if err != nil {
		t.Fatalf("get livekit: %v", err)
	}
	if lkCfg.LiveKitHost != "user-saved-host" {
		t.Errorf("livekit_host = %q, want user-saved-host (existing preserved)", lkCfg.LiveKitHost)
	}

	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active provider: %v", err)
	}
	if active != "livekit" {
		t.Errorf("active provider = %q, want livekit", active)
	}
}

func TestSyncFromEnv_SeedsWhenDBEmpty(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{
		SFUProvider: "livekit",
		LiveKitHost: "ws://livekit:7880",
		LiveKitKey:  "key",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync from env: %v", err)
	}

	cfg, err := repo.GetByProvider("livekit")
	if err != nil {
		t.Fatalf("livekit not seeded: %v", err)
	}
	if cfg.LiveKitHost != "ws://livekit:7880" {
		t.Errorf("livekit_host = %q, want env value", cfg.LiveKitHost)
	}

	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active provider: %v", err)
	}
	if active != "livekit" {
		t.Errorf("active provider = %q, want livekit", active)
	}
}

func TestUpdateFromDTO_PersistsLiveKitConfig(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{
		SFUProvider: "livekit",
		LiveKitHost: "wss://livekit.example.com",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync from env: %v", err)
	}

	cfg, err := svc.UpdateFromDTO(&UpdateSFUConfigRequest{
		Provider:    "livekit",
		LiveKitHost: "wss://voice.example.com",
		LiveKitKey:  "api-key",
	})
	if err != nil {
		t.Fatalf("update livekit: %v", err)
	}
	if cfg.LiveKitHost != "wss://voice.example.com" {
		t.Errorf("livekit_host = %q, want updated value", cfg.LiveKitHost)
	}

	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "livekit" {
		t.Errorf("active = %q, want livekit", active)
	}
}

func TestUpdateFromDTO_RejectsUnsupportedProvider(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	svc := NewSFUConfigService(repo, &config.Config{SFUProvider: "livekit"})
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	_, err := svc.UpdateFromDTO(&UpdateSFUConfigRequest{Provider: "removed"})
	if err == nil {
		t.Fatal("update to removed provider must fail")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS AppError, got %v", err)
	}

	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "livekit" {
		t.Fatalf("active = %q, want livekit preserved", active)
	}
}

func TestSwitchProvider_NoConfigChange(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{
		SFUProvider: "livekit",
		LiveKitHost: "lk-host",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	lkCfg, err := svc.SwitchProvider("livekit")
	if err != nil {
		t.Fatalf("switch to livekit: %v", err)
	}
	if lkCfg.LiveKitHost != "lk-host" {
		t.Errorf("livekit_host = %q, want lk-host", lkCfg.LiveKitHost)
	}

	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "livekit" {
		t.Errorf("active = %q, want livekit", active)
	}
}

func TestSwitchProvider_RejectsUnknownProvider(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{SFUProvider: "livekit"}
	svc := NewSFUConfigService(repo, baseCfg)
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	_, err := svc.SwitchProvider("removed")
	if err == nil {
		t.Fatal("switch to removed provider must fail")
	}
	var appErr *pkg.AppError
	if !errors.As(err, &appErr) || appErr.Code != pkg.INVALID_PARAMS {
		t.Fatalf("expected INVALID_PARAMS AppError, got %v", err)
	}
}

func TestSyncFromEnv_UnknownEnvProvider_DoesNotFail(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	if err := NewSFUConfigService(repo, &config.Config{SFUProvider: "livekit"}).SyncFromEnv(); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	svc := NewSFUConfigService(repo, &config.Config{SFUProvider: "removed"})
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync with unknown provider must not abort startup, got %v", err)
	}

	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "livekit" {
		t.Fatalf("active = %q, want livekit fallback (unknown env provider must not be activated)", active)
	}
}

func TestSyncFromEnv_FallsBackWhenCurrentActiveIsDisabled(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	if err := NewSFUConfigService(repo, &config.Config{SFUProvider: "livekit"}).SyncFromEnv(); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	// 模拟旧库遗留的未知激活状态。
	if err := repo.SetActiveProvider("removed"); err != nil {
		t.Fatalf("seed unknown active provider: %v", err)
	}

	svc := NewSFUConfigService(repo, &config.Config{SFUProvider: "removed"})
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync with unknown env provider must not abort startup, got %v", err)
	}

	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "livekit" {
		t.Fatalf("active = %q, want livekit fallback (unknown active must not be preserved)", active)
	}
}

func TestListProviders_ReturnsLiveKitOnly(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{SFUProvider: "livekit"}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	cfgs, active, err := svc.ListProviders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if active != "livekit" {
		t.Errorf("active = %q, want livekit", active)
	}
	if len(cfgs) != 1 || cfgs[0].Provider != "livekit" {
		t.Errorf("providers = %+v, want only livekit", cfgs)
	}
}

func TestListProviders_FiltersLegacyRows(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	if err := repo.Save(&model.SFUConfig{Provider: "livekit", LiveKitHost: "h"}); err != nil {
		t.Fatalf("seed livekit: %v", err)
	}
	// 旧版本遗留的行
	if err := repo.Save(&model.SFUConfig{Provider: "legacy"}); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	svc := NewSFUConfigService(repo, &config.Config{SFUProvider: "livekit"})
	cfgs, _, err := svc.ListProviders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(cfgs) != 1 || cfgs[0].Provider != "livekit" {
		t.Fatalf("providers = %+v, want legacy row filtered out", cfgs)
	}
}

func TestGet_FallsBackWhenActiveIsLegacy(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	if err := repo.Save(&model.SFUConfig{Provider: "legacy"}); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if err := repo.SetActiveProvider("legacy"); err != nil {
		t.Fatalf("set legacy active: %v", err)
	}

	svc := NewSFUConfigService(repo, &config.Config{SFUProvider: "livekit", LiveKitHost: "env-host"})
	cfg, err := svc.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if cfg.Provider != "livekit" {
		t.Fatalf("provider = %q, want livekit", cfg.Provider)
	}

	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "livekit" {
		t.Fatalf("active = %q, want livekit (legacy active must be repaired)", active)
	}
}

func TestResolveConfig_EnvOverridesActiveProvider(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{
		SFUProvider: "livekit",
		LiveKitHost: "env-host",
		LiveKitKey:  "env-key",
		// LiveKitSecret 故意留空 → 应回退到 DB
	}
	svc := NewSFUConfigService(repo, baseCfg)
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	// 模拟用户在 DB 中保存的 livekit 配置
	if err := repo.Save(&model.SFUConfig{
		Provider:      "livekit",
		LiveKitHost:   "db-host",
		LiveKitKey:    "db-key",
		LiveKitSecret: "db-secret",
	}); err != nil {
		t.Fatalf("save livekit: %v", err)
	}

	resolved, err := svc.ResolveConfig()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.SFUProvider != "livekit" {
		t.Errorf("provider = %q, want livekit", resolved.SFUProvider)
	}
	if resolved.LiveKitHost != "env-host" {
		t.Errorf("livekit_host = %q, want env-host (env overrides)", resolved.LiveKitHost)
	}
	if resolved.LiveKitKey != "env-key" {
		t.Errorf("livekit_key = %q, want env-key (env overrides)", resolved.LiveKitKey)
	}
	if resolved.LiveKitSecret != "db-secret" {
		t.Errorf("livekit_secret = %q, want db-secret (env empty -> DB fallback)", resolved.LiveKitSecret)
	}
}

func TestUpdateFromDTO_KeepsExistingSecrets(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	svc := NewSFUConfigService(repo, &config.Config{SFUProvider: "livekit"})

	if _, err := svc.UpdateFromDTO(&UpdateSFUConfigRequest{
		Provider:      "livekit",
		LiveKitHost:   "wss://livekit.example.com",
		LiveKitKey:    "key-1",
		LiveKitSecret: "super-secret-value",
	}); err != nil {
		t.Fatalf("seed update: %v", err)
	}

	cfg, err := svc.UpdateFromDTO(&UpdateSFUConfigRequest{
		Provider:    "livekit",
		LiveKitHost: "wss://livekit.example.com",
		LiveKitKey:  "key-1",
		// LiveKitSecret 留空，应保留旧值
	})
	if err != nil {
		t.Fatalf("update keep secret: %v", err)
	}
	if cfg.LiveKitSecret != "super-secret-value" {
		t.Fatalf("secret = %q, want preserved", cfg.LiveKitSecret)
	}

	pub := ToPublicSFUConfig(cfg)
	if pub.LiveKitSecret != "" {
		t.Fatalf("public secret should be empty, got %q", pub.LiveKitSecret)
	}
	if !pub.LiveKitSecretSet {
		t.Fatal("public secret_set should be true")
	}
}
