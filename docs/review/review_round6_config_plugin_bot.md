# GOSpeak Caveman Review — Round 6 (Config / Plugin / Storage / Bot / OAuth) — 二次验证版

Generated: 2026-08-05
Scope: config/*, plugin/*, service/storage.go, service/conversation.go, service/oauth.go, packages/bot/*, storage/*

---

## 二次验证结论

| # | Severity | File:Line | Finding | 验证结果 |
|---|----------|-----------|---------|----------|
| C1 | 🟡 | `config.go:18,476` | JWTKey 默认 "default-secret"；生产+无Redis时强制检查，Redis存在时无限制 | ✅ 部分缓解（仍有gap）|
| C2 | 🟡 | `config.go:70` | CORSOrigin 默认 "*"（Round 1 已报）| ✅ 确认 |
| **S1** | **🔴** | **storage_service.go:77** | **UpdateConfig 复用配置时 err 非 nil 非 NotFound 时静默继续，密钥被清空** | **✅ 确认（BUG）** |
| S2 | ✅ | `storage_service.go:55` | GetConfig 用 errors.Is（正确）| ✅ 确认 |
| S3 | ✅ | `storage/local.go:resolvePath` | 路径穿越检测正确 | ✅ 确认 |
| P1 | 🔵 | `plugin/registry.go` | Get/Names 不受 lifecycle lock 保护 | ✅ 确认（设计选择）|
| P2 | 🔵 | `plugin/types.go` | 无版本兼容性检查 | ✅ 确认 |
| O1 | 🔵 | `oauth_service.go` | GetAuthURL error 处理正确 | ✅ 确认 |
| B1 | 🔵 | `bot/main.ts:68` | 异常时未 stop runner | ✅ 确认 |
| B2 | ✅ | `bot/botRunner.ts:257` | stop() 正确清理顺序 | ✅ 确认 |

---

## 🔴 Bug — storage_service.go:77: UpdateConfig 复用配置 error 未处理

**代码：**
```go
func (s *StorageService) UpdateConfig(cfg model.StorageConfig) error {
    existing, err := s.repo.GetConfig()
    if err == nil && existing != nil {
        cfg.AccessKey = pkg.KeepSecret(cfg.AccessKey, existing.AccessKey)
        cfg.SecretKey = pkg.KeepSecret(cfg.SecretKey, existing.SecretKey)
    }
    // err != nil 但不是 ErrRecordNotFound 时：代码继续执行，existing 为 nil，
    // cfg.AccessKey/SecretKey 使用请求值（可能为空字符串），覆盖数据库中的正确密钥！
    if err := s.repo.SaveConfig(&cfg); err != nil {
        return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
    }
    ...
}
```

**Bug 流程：**
1. 用户已有 S3 配置（access_key="AKIAXXX", secret_key="SECRET"）
2. DB 查询失败（非 NotFound 的其他错误），`existing == nil`
3. `cfg.AccessKey`/`cfg.SecretKey` 使用请求值（前端传空或不传）
4. `SaveConfig` 将空字符串写入 DB，**原有密钥被清空**，S3 上传永久失败

**修复：**
```go
existing, err := s.repo.GetConfig()
if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
    return pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
}
if existing != nil {
    cfg.AccessKey = pkg.KeepSecret(cfg.AccessKey, existing.AccessKey)
    cfg.SecretKey = pkg.KeepSecret(cfg.SecretKey, existing.SecretKey)
}
```

---

## 🟡 Risk — config.go:18,476: JWT default-secret 部分缓解

**现状：**
```go
// config.go:476-477
if key == "" || key == "default-secret" {
    errs = append(errs, "JWT_KEY is required in production when REDIS_HOST is empty")
}
```

**部分缓解**：当 `REDIS_HOST` 为空（无 Redis）时，强制要求显式设置 `JWT_KEY`。

**Gap**：当 Redis 存在时，使用 `default-secret` 不会产生错误或警告。Redis 的 `jwt_key` 轮转会掩盖这个问题，但若 Redis 不可用或 key rotation 未配置，仍会用不安全默认值。

**建议**：统一检查，不限 Redis：
```go
if key == "" || key == "default-secret" {
    errs = append(errs, "JWT_KEY must be set to a non-default value")
}
```

---

## 待修复项（Round 6 新增 + 累计）

### 🔴 新增

| # | 文件:行 | 描述 |
|---|---------|------|
| B-S1 | `storage_service.go:77` | UpdateConfig 复用配置 error 未处理导致密钥被清空 |

### 🟡 新增

| # | 文件:行 | 描述 |
|---|---------|------|
| R-C1b | `config.go:476` | JWT default-secret 检查仅限无 Redis 场景，Redis 存在时无限制 |

### 🔵 新增

| # | 文件:行 | 描述 |
|---|---------|------|
| N-P1 | `plugin/registry.go` | Get/Names 不受 lifecycle lock 保护 |
| N-P2 | `plugin/types.go` | 无版本兼容性检查 |
| N-B1 | `bot/main.ts:68` | 异常时未 stop runner |
