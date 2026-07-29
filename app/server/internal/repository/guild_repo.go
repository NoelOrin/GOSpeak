package repository

import (
	"GOSpeak/internal/model"

	"gorm.io/gorm"
)

type GuildRepository struct {
	db *gorm.DB
}

func NewGuildRepository(db *gorm.DB) *GuildRepository {
	return &GuildRepository{db: db}
}

func (r *GuildRepository) Create(guild *model.Guild) error {
	return r.db.Create(guild).Error
}

func (r *GuildRepository) GetByUUID(uuid string) (*model.Guild, error) {
	var guild model.Guild
	err := r.db.Where("uuid = ?", uuid).First(&guild).Error
	return &guild, err
}

func (r *GuildRepository) GetByInviteCode(code string) (*model.Guild, error) {
	var guild model.Guild
	err := r.db.Where("invite_code = ?", code).First(&guild).Error
	return &guild, err
}

func (r *GuildRepository) List(page, pageSize int) ([]model.Guild, int64, error) {
	var guilds []model.Guild
	var total int64
	r.db.Model(&model.Guild{}).Count(&total)
	err := r.db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&guilds).Error
	return guilds, total, err
}

func (r *GuildRepository) ListPublic(page, pageSize int) ([]model.Guild, int64, error) {
	var guilds []model.Guild
	var total int64
	r.db.Model(&model.Guild{}).Where("is_public = ?", true).Count(&total)
	err := r.db.Where("is_public = ?", true).Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&guilds).Error
	return guilds, total, err
}

func (r *GuildRepository) Update(guild *model.Guild) error {
	return r.db.Save(guild).Error
}

func (r *GuildRepository) Delete(uuid string) error {
	return r.db.Where("uuid = ?", uuid).Delete(&model.Guild{}).Error
}

// --- GuildMember ---

func (r *GuildRepository) AddMember(member *model.GuildMember) error {
	return r.db.Create(member).Error
}

func (r *GuildRepository) UpdateMember(member *model.GuildMember) error {
	return r.db.Save(member).Error
}

func (r *GuildRepository) RemoveMember(guildUUID, userUUID string) error {
	return r.db.Where("guild_uuid = ? AND user_uuid = ?", guildUUID, userUUID).Delete(&model.GuildMember{}).Error
}

func (r *GuildRepository) GetMember(guildUUID, userUUID string) (*model.GuildMember, error) {
	var member model.GuildMember
	err := r.db.Where("guild_uuid = ? AND user_uuid = ?", guildUUID, userUUID).First(&member).Error
	return &member, err
}

func (r *GuildRepository) ListMembers(guildUUID string) ([]model.GuildMember, error) {
	var members []model.GuildMember
	err := r.db.Where("guild_uuid = ?", guildUUID).Order("joined_at ASC").Find(&members).Error
	return members, err
}

func (r *GuildRepository) ListUserGuilds(userUUID string) ([]string, error) {
	var guildUUIDs []string
	err := r.db.Model(&model.GuildMember{}).Where("user_uuid = ?", userUUID).Pluck("guild_uuid", &guildUUIDs).Error
	return guildUUIDs, err
}

func (r *GuildRepository) CountMembers(guildUUID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.GuildMember{}).Where("guild_uuid = ?", guildUUID).Count(&count).Error
	return count, err
}

func (r *GuildRepository) CountRooms(guildUUID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Room{}).Where("guild_uuid = ?", guildUUID).Count(&count).Error
	return count, err
}
