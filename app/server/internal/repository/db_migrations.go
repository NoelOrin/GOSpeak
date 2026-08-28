package repository

import (
	"GOSpeak/internal/model"
	"GOSpeak/internal/pkg"
	"fmt"
	"strings"

	"github.com/nrednav/cuid2"
	"gorm.io/gorm"
)

// migrateStorageConfigSchema 修复 storage_configs 上不安全的 GORM default。
// 旧模型把 allowed_types / path_prefix 写成 gorm default，逗号与斜杠会被
// glebarez migrator 截断，生成非法 CREATE TABLE ...__temp SQL 并 panic。
// 这里在 AutoMigrate 前把表重建为无危险 default 的干净结构，并保留已有数据。
func migrateStorageConfigSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable("storage_configs") {
		return nil
	}

	var ddl string
	if err := db.Raw(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		"table", "storage_configs",
	).Scan(&ddl).Error; err != nil || ddl == "" {
		// 非 SQLite 或无法读取 sqlite_master：跳过，交给 AutoMigrate。
		return nil
	}

	// 旧 default 或非 gorm 风格 DDL（无反引号列名）都会让 glebarez migrator 解析失败。
	needsRebuild := strings.Contains(ddl, "image/jpeg,image/png") ||
		strings.Contains(ddl, `DEFAULT "uploads/"`) ||
		strings.Contains(ddl, "DEFAULT 'uploads/'") ||
		strings.Contains(ddl, "allowed_types text DEFAULT") ||
		strings.Contains(ddl, "`allowed_types` text DEFAULT") ||
		!strings.Contains(ddl, "`provider_type`")
	if !needsRebuild {
		return nil
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	// 使用 glebarez/gORM 常见的反引号单行 DDL，避免 migrator 解析多行/无引号列名失败。
	if err := tx.Exec("CREATE TABLE `storage_configs__rebuild` (`id` integer PRIMARY KEY AUTOINCREMENT,`provider_type` text,`endpoint` text,`bucket` text,`region` text,`access_key` text,`secret_key` text,`public_base_url` text,`path_prefix` text,`max_file_size` integer,`allowed_types` text,`created_at` datetime,`updated_at` datetime)").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate storage_configs schema: create rebuild table: %w", err)
	}
	if err := tx.Exec(`INSERT INTO storage_configs__rebuild (
	id, provider_type, endpoint, bucket, region, access_key, secret_key,
	public_base_url, path_prefix, max_file_size, allowed_types, created_at, updated_at
)
SELECT
	id, provider_type, endpoint, bucket, region, access_key, secret_key,
	public_base_url, path_prefix, max_file_size, allowed_types, created_at, updated_at
FROM storage_configs`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate storage_configs schema: copy data: %w", err)
	}
	if err := tx.Exec(`DROP TABLE storage_configs`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate storage_configs schema: drop old table: %w", err)
	}
	if err := tx.Exec(`ALTER TABLE storage_configs__rebuild RENAME TO storage_configs`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate storage_configs schema: rename rebuild table: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("migrate storage_configs schema: commit: %w", err)
	}
	return nil
}

// migrateUserSearchIndexes 为 PostgreSQL 用户搜索列创建 trigram GIN 索引，
// 避免 List keyword 的 %kw% 模糊查询全表扫描；SQLite/MySQL 不做变更。
func migrateUserSearchIndexes(db *gorm.DB, dbType DatabaseEnum) error {
	if dbType != PostgreSQL {
		return nil
	}
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
		return fmt.Errorf("create pg_trgm extension: %w", err)
	}
	for _, idx := range []struct{ name, column string }{
		{"idx_users_name_trgm", "name"},
		{"idx_users_display_name_trgm", "display_name"},
		{"idx_users_email_trgm", "email"},
	} {
		if db.Migrator().HasIndex("users", idx.name) {
			continue
		}
		if err := db.Exec(fmt.Sprintf("CREATE INDEX %s ON users USING gin (%s gin_trgm_ops)", idx.name, idx.column)).Error; err != nil {
			return fmt.Errorf("create %s: %w", idx.name, err)
		}
	}
	return nil
}

// migrateOldSFUConfig handles migration from the old single-row sfu_configs
// table (id as PK) to the new per-provider schema (provider as PK).
// Detection uses PRAGMA table_info (SQLite) to check the primary key column.

// migrateOldSFUConfig handles migration from the old single-row sfu_configs
// table (id as PK) to the new per-provider schema (provider as PK).
// Detection uses PRAGMA table_info (SQLite) to check the primary key column.
func migrateOldSFUConfig(db *gorm.DB) error {
	if !db.Migrator().HasTable("sfu_configs") {
		return nil
	}
	var pkColumn string
	if err := db.Raw("SELECT name FROM pragma_table_info(?) WHERE pk = 1", "sfu_configs").Scan(&pkColumn).Error; err != nil {
		return nil
	}
	if pkColumn != "id" {
		return nil
	}
	return db.Migrator().DropTable("sfu_configs")
}

// migrateBotTokensSchema 修复 bot_tokens DDL 中的 glebarez/modernec 不兼容问题：
// 1) `permissions_json TEXT` 无反引号列名
// 2) `DEFAULT "user"` 双引号字面量（modernc 会误解析为标识符）

// migrateBotTokensSchema 修复 bot_tokens DDL 中的 glebarez/modernec 不兼容问题：
// 1) `permissions_json TEXT` 无反引号列名
// 2) `DEFAULT "user"` 双引号字面量（modernc 会误解析为标识符）
func migrateBotTokensSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable("bot_tokens") {
		return nil
	}
	var ddl string
	if err := db.Raw(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		"table", "bot_tokens",
	).Scan(&ddl).Error; err != nil || ddl == "" {
		return nil
	}
	// 无反引号列名 或 DEFAULT "xxx" 双引号字面量
	if !strings.Contains(ddl, "permissions_json TEXT") && !strings.Contains(ddl, `DEFAULT "user"`) {
		return nil
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := tx.Exec(`CREATE TABLE "bot_tokens__rebuild" (` +
		"`id` integer PRIMARY KEY AUTOINCREMENT," +
		"`uuid` uuid," +
		"`name` text," +
		"`user_uuid` uuid," +
		"`role` text DEFAULT 'user'," +
		"`revoked` numeric DEFAULT false," +
		"`permissions` text," +
		"`expires_at` datetime," +
		"`created_at` datetime," +
		"`updated_at` datetime," +
		"`created_by` text," +
		"`last_used_at` datetime" +
		`)`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate bot_tokens: create rebuild table: %w", err)
	}
	if err := tx.Exec(`INSERT INTO "bot_tokens__rebuild" (
		id, uuid, name, user_uuid, role, revoked, permissions,
		expires_at, created_at, updated_at, created_by, last_used_at
	) SELECT
		id, uuid, name, user_uuid, role, revoked, permissions,
		expires_at, created_at, updated_at, created_by, last_used_at
	FROM bot_tokens`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate bot_tokens: copy data: %w", err)
	}
	if err := tx.Exec(`DROP TABLE bot_tokens`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate bot_tokens: drop old table: %w", err)
	}
	if err := tx.Exec(`ALTER TABLE "bot_tokens__rebuild" RENAME TO bot_tokens`).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("migrate bot_tokens: rename rebuild table: %w", err)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("migrate bot_tokens: commit: %w", err)
	}
	return nil
}

// migrateEmailVerificationCodesSchema 清理 email_verification_codes 上历史残留的
// 无反引号索引 DDL（glebarez 解析多行/无反引号索引定义时会 panic）。

// migrateEmailVerificationCodesSchema 清理 email_verification_codes 上历史残留的
// 无反引号索引 DDL（glebarez 解析多行/无反引号索引定义时会 panic）。
func migrateEmailVerificationCodesSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable("email_verification_codes") {
		return nil
	}
	// 只查 SQLite 风格索引（glebarez 仅用于 SQLite）
	if db.Migrator().HasIndex("email_verification_codes", "idx_email_codes_email_scene_created") {
		if err := db.Migrator().DropIndex("email_verification_codes", "idx_email_codes_email_scene_created"); err != nil {
			// fallback: raw SQL
			if err2 := db.Exec(`DROP INDEX IF EXISTS idx_email_codes_email_scene_created`).Error; err2 != nil {
				return fmt.Errorf("migrate email_verification_codes: drop legacy index: %w", err2)
			}
		}
	}
	return nil
}

func migrateDomainCUID2(db *gorm.DB) error {
	type domainRow struct {
		ID   uint
		UUID string
	}
	var rows []domainRow
	if err := db.Table("domains").Select("id, uuid").Find(&rows).Error; err != nil {
		return fmt.Errorf("load domains for cuid migration: %w", err)
	}
	for _, row := range rows {
		if !strings.Contains(row.UUID, "-") {
			continue
		}
		next := cuid2.Generate()
		if err := db.Table("domains").Where("id = ?", row.ID).Update("uuid", next).Error; err != nil {
			return fmt.Errorf("migrate domain id %d: %w", row.ID, err)
		}
		if err := db.Table("domain_members").Where("domain_uuid = ?", row.UUID).Update("domain_uuid", next).Error; err != nil {
			return fmt.Errorf("migrate domain_members for %s: %w", row.UUID, err)
		}
		if err := db.Table("room").Where("domain_uuid = ?", row.UUID).Update("domain_uuid", next).Error; err != nil {
			return fmt.Errorf("migrate rooms for %s: %w", row.UUID, err)
		}
	}
	return nil
}

func autoMigrate() error {
	if err := getDB().AutoMigrate(
		&model.Role{},
		&model.User{},
		&model.Room{},
		&model.UserGroup{},
		&model.OAuthProvider{},
		&model.OAuthAccount{},
		&model.EmailConfig{},
		&model.EmailVerificationCode{},
		&model.SFUConfig{},
		&model.SFUActiveProvider{},
		&model.Permission{},
		&model.RolePermission{},
		&model.Mute{},
		&model.StorageConfig{},
		&model.BotToken{},
		&model.PluginConfig{},
		&model.Message{},
		&model.MessageReaction{},
		&model.MessageMention{},
		&model.Domain{},
		&model.DomainMember{},
		&model.DomainRole{},
		&model.DomainRolePermission{},
		&model.ClusterNode{},
		&model.ServerAssignment{},
		&model.ClusterLeaderFence{},
		&model.AuditLog{},
		&model.DomainGuestBan{},
	); err != nil {
		return err
	}
	return migrateConversationQueryIndexes(getDB())
}

// migrateConversationQueryIndexes 确保会话列表游标查询依赖的表与复合索引存在。
func migrateConversationQueryIndexes(db *gorm.DB) error {
	m := db.Migrator()
	if !m.HasTable("conversation_participants") {
		if err := m.CreateTable(&model.ConversationParticipant{}); err != nil {
			return err
		}
	}
	if !m.HasIndex("conversation_participants", "idx_conv_part_identity") {
		return db.Exec("CREATE INDEX idx_conv_part_identity ON conversation_participants (identity_a, identity_b)").Error
	}
	return nil
}

// migrateRoomPasswords 将存量明文房间密码升级为 bcrypt 哈希。

// migrateMessageAuthorUUID 为存量消息回填稳定的作者 UUID，改名后历史消息仍可被本人编辑/删除。
func migrateMessageAuthorUUID(db *gorm.DB) error {
	if !db.Migrator().HasColumn("messages", "author_uuid") {
		return nil
	}
	return db.Exec(`UPDATE messages SET author_uuid = (SELECT uuid FROM users WHERE users.name = messages.author_id) WHERE author_uuid = '' OR author_uuid IS NULL`).Error
}

// migrateRoomPasswords 将存量明文房间密码升级为 bcrypt 哈希。
func migrateRoomPasswords(db *gorm.DB) error {
	var rooms []model.Room
	if err := db.Find(&rooms).Error; err != nil {
		return err
	}
	for i := range rooms {
		if rooms[i].Password == "" || pkg.IsHashedPassword(rooms[i].Password) {
			continue
		}
		hash, err := pkg.HashPassword(rooms[i].Password)
		if err != nil {
			return err
		}
		if err := db.Model(&model.Room{}).Where("uuid = ?", rooms[i].UUID).Update("password", hash).Error; err != nil {
			return err
		}
	}
	return nil
}

// EnsureDefaultDomain 在管理员用户就绪后创建/修复默认 Domain。

// EnsureDefaultDomain 在管理员用户就绪后创建/修复默认 Domain。
func EnsureDefaultDomain(db *gorm.DB, ownerUUID string) error {
	if ownerUUID == "" {
		return fmt.Errorf("default domain owner uuid is required")
	}
	return migrateDefaultDomain(db, ownerUUID)
}

// migrateDefaultDomain 创建默认 Domain、补齐 owner/成员；仅在首次创建默认 Domain 时，
// 将无归属的存量房间归入其中。

// migrateDefaultDomain 创建默认 Domain、补齐 owner/成员；仅在首次创建默认 Domain 时，
// 将无归属的存量房间归入其中。
func migrateDefaultDomain(db *gorm.DB, ownerUUID string) error {
	var existing []model.Domain
	if err := db.Where("name = ?", "默认域").Find(&existing).Error; err != nil {
		return err
	}
	var defaultDomain *model.Domain
	if len(existing) > 0 {
		defaultDomain = &existing[0]
	}
	if defaultDomain == nil {
		var count int64
		if err := db.Model(&model.Domain{}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		defaultDomain = &model.Domain{
			Name: "默认域", Description: "系统默认语音域", IsPublic: false, OwnerUUID: ownerUUID,
		}
		if err := db.Create(defaultDomain).Error; err != nil {
			return err
		}
		// 仅在默认 Domain 首次创建时迁移平台房，避免重启后改变 domain_uuid="" 的语义。
		if err := db.Model(&model.Room{}).Where("domain_uuid = ?", "").
			Update("domain_uuid", defaultDomain.UUID).Error; err != nil {
			return err
		}
	} else if defaultDomain.OwnerUUID == "" {
		if err := db.Model(&model.Domain{}).Where("uuid = ?", defaultDomain.UUID).Update("owner_uuid", ownerUUID).Error; err != nil {
			return err
		}
		defaultDomain.OwnerUUID = ownerUUID
	}

	ownerUUIDForMember := defaultDomain.OwnerUUID
	if ownerUUIDForMember == "" {
		ownerUUIDForMember = ownerUUID
	}
	var member model.DomainMember
	err := db.Where("domain_uuid = ? AND user_uuid = ?", defaultDomain.UUID, ownerUUIDForMember).First(&member).Error
	if err == gorm.ErrRecordNotFound {
		member = model.DomainMember{DomainUUID: defaultDomain.UUID, UserUUID: ownerUUIDForMember, RoleName: "owner"}
		if err := db.Create(&member).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if member.RoleName != "owner" {
		if err := db.Model(&model.DomainMember{}).Where("domain_uuid = ? AND user_uuid = ?", defaultDomain.UUID, ownerUUIDForMember).Update("role_name", "owner").Error; err != nil {
			return err
		}
	}

	if err := SeedDefaultDomainRoles(db, defaultDomain.UUID); err != nil {
		return fmt.Errorf("seed default domain roles: %w", err)
	}
	return nil
}
