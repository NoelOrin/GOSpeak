# GOSpeak Caveman Review — Round 3 (Repository / Service / Auth) — 二次验证版

Generated: 2026-08-05
Scope: repository/*, service/auth.go, service/mute.go, service/user.go

---

## 二次验证结论

| # | Severity | File:Line | Finding | 验证结果 |
|---|----------|-----------|---------|----------|
| B-A1 | 🔴 | auth_service.go:89 | Register GetByName 忽略 error | ✅ 确认（BUG）|
| B-A2 | 🔴 | auth_service.go:288 | 同上第二处 | ✅ 确认（BUG）|
| B-A3 | 🟡 | mute_service.go:40 | DeleteByUserID 失败仍 notifyExpired | ✅ 确认（小问题）|
| B-A4 | ❌ | user_repo.go:48 | GetByNames error 返回 nil 导致 panic | ❌ 误报（enrichMembers 先初始化空 map 再条件替换，安全）|
| R-B1 | 🟡 | repository/* | gorm.ErrRecordNotFound 系统性用 `==` | ✅ 确认（系统性风险）|
| R-B2 | 🟡 | auth_service.go:67 | bcrypt CompareHashAndPassword error 时 needChange 错 | ✅ 确认（需加 err==nil 检查）|
| N-C1 | 🔵 | auth_service.go:102 | Register GetByEmail 错误处理更规范 | ✅ 确认（正面参考）|
| N-C2 | 🔵 | db.go:66 | PostgreSQL DSN hardcode myapp | ✅ 确认 |
| N-C3 | 🔵 | auth_service.go:72 | Login 无失败日志 | ✅ 确认 |

---

## 待修复项（确认版）

### 🔴 B-A1 — auth_service.go:89

```go
existing, _ := s.userRepo.GetByName(req.Username)
if existing != nil {
    return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
}
```

DB 故障时 `existing == nil`，走到 `r.db.Create(user)` 才暴露真正错误。**修复**：
```go
existing, err := s.userRepo.GetByName(req.Username)
if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
    return nil, pkg.NewAppError(pkg.INTERNAL_ERROR, err.Error())
}
if existing != nil {
    return nil, pkg.NewAppError(pkg.USERNAME_EXISTS)
}
```

### 🔴 B-A2 — auth_service.go:288

同上，修复方案一致。

### 🟡 B-A3 — mute_service.go:40

`DeleteByUserID` 失败（非 NotFound）时不应触发 `notifyExpired`。**修复**：在 `delErr != nil && !errors.Is(delErr, gorm.ErrRecordNotFound)` 时 return error，不再 notifyExpired。

### 🟡 R-B1 — repository 层系统性 `== gorm.ErrRecordNotFound`

涉及文件：
- `user_repo.go`: GetByID, GetByName, GetByEmail, GetByUUID
- `mute_service.go`: 多处
- `auth_service.go`: Login, IsMutedByIdentity

**建议**：统一改为 `errors.Is(err, gorm.ErrRecordNotFound)`。

### 🟡 R-B2 — auth_service.go:67

```go
needChange := user.Role == "admin" && bcrypt.CompareHashAndPassword(...) == nil
```

当 bcrypt 比较返回 error（非 nil）时 `needChange` 不赋值，默认为 false（不需要改密）。密码 hash 损坏时逻辑错误。**修复**：
```go
needChange := user.Role == "admin" && err == nil
```
（`err` 是 `bcrypt.CompareHashAndPassword` 的返回值）

---

## 新增：db.go PostgreSQL DSN 硬编码

**N-C2 — db.go:66** PostgreSQL 连接 DSN 硬编码 `dbname=myapp`，config 无 DBName 字段。生产部署时所有 PostgreSQL 实例共享 `myapp` 数据库。
→ 确认是否为开发默认值；生产应从 config 读取 `DB_NAME`。
