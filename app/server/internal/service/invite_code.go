package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"time"

	"GOSpeak/internal/pkg"
)

// inviteAlphabet 邀请码字符集：32 个无歧义字符（去掉 I、O、0、1），
// 正好满足 base32 的字母表长度要求。
const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var inviteEncoding = base32.NewEncoding(inviteAlphabet).WithPadding(base32.NoPadding)

const (
	// invitePayloadSize 参与 HMAC 的载荷字节数：域前缀 4 + codeID 4 + 过期 4 + 次数上限 2。
	invitePayloadSize = 14
	// inviteMACSize HMAC 截断长度；64 位足以使碰撞概率可忽略。
	inviteMACSize = 8
	// inviteCodeBytes 未编码的完整字节数。
	inviteCodeBytes = invitePayloadSize + inviteMACSize
	// inviteCodeLength base32 编码后的字符数：ceil(22*8/5) = 36。
	inviteCodeLength = 36
	// inviteKeySalt 与存储加密用途隔离的固定 salt 前缀；算法升级时追加 v2。
	inviteKeySalt = "gospeak:invite:v1"
	// InviteMaxUsesLimit 次数上限的编码上限（uint16）。
	InviteMaxUsesLimit = 65535
)

var (
	// ErrInviteCodeMalformed 码格式错误（长度、字符集、base32 解码失败）。
	ErrInviteCodeMalformed = pkg.NewAppError(pkg.INVALID_PARAMS, "invite code format is invalid")
	// ErrInviteCodeInvalid 验签失败（伪造或篡改）。
	ErrInviteCodeInvalid = pkg.NewAppError(pkg.INVALID_PARAMS, "invite code is invalid")
	// ErrInviteCodeExpired 码已过期。
	ErrInviteCodeExpired = pkg.NewAppError(pkg.INVALID_PARAMS, "invite code has expired")
)

// InviteCode 是邀请码验签后的明文载荷。零值 ExpiresAt 表示永不过期。
type InviteCode struct {
	DomainPrefix [4]byte
	InviteID     uint32
	ExpiresAt    time.Time
	MaxUses      uint16
}

// DeriveInviteKey 从存储加密密钥派生邀请码 HMAC 密钥。
// StorageEncryptKey 为空时回退 JWTKey（开发环境），生产必设 StorageEncryptKey。
func DeriveInviteKey(storageEncryptKey, jwtKey string) []byte {
	base := storageEncryptKey
	if base == "" {
		base = jwtKey
	}
	mac := hmac.New(sha256.New, []byte(base))
	mac.Write([]byte(inviteKeySalt))
	return mac.Sum(nil)
}

// DomainPrefixOf 返回域 UUID 的 4 字节前缀（SHA-256 截断），用于验签后定位域而不泄露 UUID。
func DomainPrefixOf(domainUUID string) [4]byte {
	sum := sha256.Sum256([]byte(domainUUID))
	var out [4]byte
	copy(out[:], sum[:4])
	return out
}

// EncodeInviteCode 生成 36 字符邀请码。
func EncodeInviteCode(key []byte, code InviteCode) (string, error) {
	// MaxUses 是 uint16，65535 上限由类型保证；服务层在接收 int 时另行校验。
	var payload [invitePayloadSize]byte
	copy(payload[0:4], code.DomainPrefix[:])
	binary.BigEndian.PutUint32(payload[4:8], code.InviteID)
	var expires uint32
	if !code.ExpiresAt.IsZero() {
		expires = uint32(code.ExpiresAt.Unix())
	}
	binary.BigEndian.PutUint32(payload[8:12], expires)
	binary.BigEndian.PutUint16(payload[12:14], code.MaxUses)

	full := make([]byte, inviteCodeBytes)
	copy(full, payload[:])
	mac := hmac.New(sha256.New, key)
	mac.Write(payload[:])
	copy(full[invitePayloadSize:], mac.Sum(nil)[:inviteMACSize])

	return inviteEncoding.EncodeToString(full), nil
}

// DecodeInviteCode 校验并解析邀请码。now 用于判断过期（零值 ExpiresAt 永不过期）。
func DecodeInviteCode(key []byte, raw string, now time.Time) (*InviteCode, error) {
	if len(raw) != inviteCodeLength {
		return nil, ErrInviteCodeMalformed
	}
	for _, r := range raw {
		if !containsInviteChar(r) {
			return nil, ErrInviteCodeMalformed
		}
	}
	full, err := inviteEncoding.DecodeString(raw)
	if err != nil || len(full) != inviteCodeBytes {
		return nil, ErrInviteCodeMalformed
	}

	payload := full[:invitePayloadSize]
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	if !hmac.Equal(full[invitePayloadSize:], mac.Sum(nil)[:inviteMACSize]) {
		return nil, ErrInviteCodeInvalid
	}

	code := &InviteCode{InviteID: binary.BigEndian.Uint32(payload[4:8])}
	copy(code.DomainPrefix[:], payload[0:4])
	if expires := binary.BigEndian.Uint32(payload[8:12]); expires != 0 {
		code.ExpiresAt = time.Unix(int64(expires), 0).UTC()
		if now.After(code.ExpiresAt) {
			return nil, ErrInviteCodeExpired
		}
	}
	code.MaxUses = binary.BigEndian.Uint16(payload[12:14])
	return code, nil
}

func containsInviteChar(r rune) bool {
	for _, c := range inviteAlphabet {
		if c == r {
			return true
		}
	}
	return false
}
