package service

import (
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
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

func TestSyncFromEnv_SeedsProvidersNotInDB(t *testing.T) {
	repo := newSFUConfigTestRepo(t)

	// 预置一条 LiveKit 配置（模拟用户已保存）
	if err := repo.Save(&model.SFUConfig{
		Provider:    "livekit",
		LiveKitHost: "user-saved-host",
	}); err != nil {
		t.Fatalf("seed livekit row: %v", err)
	}

	baseCfg := &config.Config{
		SFUProvider: "srs",
		SRSHost:     "env-default-host",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync from env: %v", err)
	}

	// LiveKit 已存在，不应被 env 覆盖
	lkCfg, err := repo.GetByProvider("livekit")
	if err != nil {
		t.Fatalf("get livekit: %v", err)
	}
	if lkCfg.LiveKitHost != "user-saved-host" {
		t.Errorf("livekit_host = %q, want user-saved-host (existing preserved)", lkCfg.LiveKitHost)
	}

	// SRS 无记录，应由 env 默认值创建
	srsCfg, err := repo.GetByProvider("srs")
	if err != nil {
		t.Fatalf("get srs: %v", err)
	}
	if srsCfg.SRSHost != "env-default-host" {
		t.Errorf("srs_host = %q, want env-default-host", srsCfg.SRSHost)
	}

	// 激活 provider 应为 env 指定的 srs
	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active provider: %v", err)
	}
	if active != "srs" {
		t.Errorf("active provider = %q, want srs", active)
	}
}

func TestSyncFromEnv_SeedsWhenDBEmpty(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{
		SFUProvider: "srs",
		SRSHost:     "localhost",
		SRSApiPort:  "1985",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync from env: %v", err)
	}

	// 所有 provider 都应被 seed
	providers := []string{"livekit", "agora", "srs", "cloudflare"}
	for _, p := range providers {
		_, err := repo.GetByProvider(p)
		if err != nil {
			t.Errorf("provider %q not seeded: %v", p, err)
		}
	}

	// 激活 provider 为 env 值
	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active provider: %v", err)
	}
	if active != "srs" {
		t.Errorf("active provider = %q, want srs", active)
	}
}

func TestUpdateFromDTO_PerProviderPersistence(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{
		SFUProvider: "livekit",
		LiveKitHost: "wss://livekit.example.com",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	// 先同步一次（seed）
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync from env: %v", err)
	}

	// 更新 SRS 配置
	srsCfg, err := svc.UpdateFromDTO(&UpdateSFUConfigRequest{
		Provider:   "srs",
		SRSHost:    "192.168.1.100",
		SRSApiPort: "1985",
	})
	if err != nil {
		t.Fatalf("update srs: %v", err)
	}
	if srsCfg.SRSHost != "192.168.1.100" {
		t.Errorf("srs_host = %q, want 192.168.1.100", srsCfg.SRSHost)
	}

	// 验证 LiveKit 配置未被覆盖
	lkCfg, err := repo.GetByProvider("livekit")
	if err != nil {
		t.Fatalf("get livekit: %v", err)
	}
	if lkCfg.LiveKitHost != "wss://livekit.example.com" {
		t.Errorf("livekit_host = %q, want original (should not be overwritten)", lkCfg.LiveKitHost)
	}

	// 激活应为 srs
	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "srs" {
		t.Errorf("active = %q, want srs", active)
	}
}

func TestUpdateFromDTO_SwitchProviderPreservesOther(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{SFUProvider: "livekit"}
	svc := NewSFUConfigService(repo, baseCfg)

	// Seed from env
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	// 配置 SRS
	_, err := svc.UpdateFromDTO(&UpdateSFUConfigRequest{
		Provider:   "srs",
		SRSHost:    "srs-host",
		SRSApiPort: "1985",
	})
	if err != nil {
		t.Fatalf("update srs: %v", err)
	}

	// 配置 LiveKit（切换回去）
	_, err = svc.UpdateFromDTO(&UpdateSFUConfigRequest{
		Provider:    "livekit",
		LiveKitHost: "lk-host",
	})
	if err != nil {
		t.Fatalf("update livekit: %v", err)
	}

	// SRS 配置应仍然保留
	srsCfg, err := repo.GetByProvider("srs")
	if err != nil {
		t.Fatalf("get srs: %v", err)
	}
	if srsCfg.SRSHost != "srs-host" {
		t.Errorf("srs_host = %q, want srs-host (should still exist)", srsCfg.SRSHost)
	}

	// 当前激活应为 livekit
	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "livekit" {
		t.Errorf("active = %q, want livekit", active)
	}
}

func TestSwitchProvider_NoConfigChange(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{
		SFUProvider: "livekit",
		LiveKitHost: "lk-host",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	// Seed + configure SRS via update
	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	_, err := svc.UpdateFromDTO(&UpdateSFUConfigRequest{
		Provider:   "srs",
		SRSHost:    "srs-host",
		SRSApiPort: "1985",
	})
	if err != nil {
		t.Fatalf("update srs: %v", err)
	}

	// 单纯切回 LiveKit，不改配置
	lkCfg, err := svc.SwitchProvider("livekit")
	if err != nil {
		t.Fatalf("switch to livekit: %v", err)
	}
	if lkCfg.LiveKitHost != "lk-host" {
		t.Errorf("livekit_host = %q, want lk-host", lkCfg.LiveKitHost)
	}

	// 激活应为 livekit
	active, err := repo.GetActiveProvider()
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active != "livekit" {
		t.Errorf("active = %q, want livekit", active)
	}

	// SRS 配置不动
	srsCfg, _ := repo.GetByProvider("srs")
	if srsCfg.SRSHost != "srs-host" {
		t.Errorf("srs_host = %q, want srs-host", srsCfg.SRSHost)
	}
}

func TestListProviders_ReturnsAll(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	baseCfg := &config.Config{SFUProvider: "srs"}
	svc := NewSFUConfigService(repo, baseCfg)

	if err := svc.SyncFromEnv(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	cfgs, active, err := svc.ListProviders()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if active != "srs" {
		t.Errorf("active = %q, want srs", active)
	}
	if len(cfgs) != 4 {
		t.Errorf("provider count = %d, want 4", len(cfgs))
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

func TestResolveConfig_SRSPublicHostAndWHIP(t *testing.T) {
	repo := newSFUConfigTestRepo(t)
	// baseCfg 不提供 public/whip，确保可从 DB 回填
	baseCfg := &config.Config{
		SFUProvider:   "srs",
		SRSHost:       "",
		SRSApiPort:    "",
		SRSSecret:     "",
		SRSWHIPURL:    "",
		SRSPublicHost: "",
	}
	svc := NewSFUConfigService(repo, baseCfg)

	if _, err := svc.UpdateFromDTO(&UpdateSFUConfigRequest{
		Provider:      "srs",
		SRSHost:       "192.168.1.10",
		SRSApiPort:    "1985",
		SRSSecret:     "secret",
		SRSWHIPURL:    "/rtc/v1/whip/?app=live",
		SRSPublicHost: "https://voice.example.com",
	}); err != nil {
		t.Fatalf("update srs: %v", err)
	}

	resolved, err := svc.ResolveConfig()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.SRSHost != "192.168.1.10" {
		t.Errorf("SRSHost=%q", resolved.SRSHost)
	}
	if resolved.SRSWHIPURL != "/rtc/v1/whip/?app=live" {
		t.Errorf("SRSWHIPURL=%q", resolved.SRSWHIPURL)
	}
	if resolved.SRSPublicHost != "https://voice.example.com" {
		t.Errorf("SRSPublicHost=%q", resolved.SRSPublicHost)
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
