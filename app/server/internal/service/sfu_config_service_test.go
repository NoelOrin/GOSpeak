package service

import (
	"testing"

	"GOSpeak/internal/config"
	"GOSpeak/internal/model"
	"GOSpeak/internal/repository"

	"gorm.io/driver/sqlite"
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
	providers := []string{"livekit", "agora", "mediasoup", "srs", "daily"}
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
	if len(cfgs) != 5 {
		t.Errorf("provider count = %d, want 5", len(cfgs))
	}
}
