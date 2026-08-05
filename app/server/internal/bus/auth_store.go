package bus

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// AuthStore is multi-instance JWT auth state (blacklist + signing keys) backed by NATS KV.
// Used when Redis is unavailable so multi-instance still shares logout/key rotation state.
type AuthStore struct {
	kv  nats.KeyValue
	nc  *nats.Conn
	own bool
	mu  sync.Mutex
}

type AuthStoreConfig struct {
	URL    string
	Prefix string
	NC     *nats.Conn
	// BucketTTL MaxAge for auth KV; zero => 7d (refresh token window).
	BucketTTL time.Duration
}

func OpenAuthStore(cfg AuthStoreConfig) (*AuthStore, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	if cfg.BucketTTL <= 0 {
		cfg.BucketTTL = 7 * 24 * time.Hour
	}
	var nc *nats.Conn
	var err error
	own := false
	if cfg.NC != nil {
		nc = cfg.NC
	} else {
		if strings.TrimSpace(cfg.URL) == "" {
			return nil, fmt.Errorf("auth store: empty URL and nil NC")
		}
		nc, err = nats.Connect(cfg.URL, nats.Name(cfg.Prefix+"-auth"), nats.Timeout(2*time.Second))
		if err != nil {
			return nil, fmt.Errorf("auth store connect: %w", err)
		}
		own = true
	}
	js, err := nc.JetStream()
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("auth store jetstream: %w", err)
	}
	bucket := cfg.Prefix + "_auth"
	kv, err := js.KeyValue(bucket)
	if err != nil {
		kv, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket: bucket,
			TTL:    cfg.BucketTTL,
		})
		if err != nil {
			if own {
				nc.Close()
			}
			return nil, fmt.Errorf("auth store kv: %w", err)
		}
	}
	return &AuthStore{kv: kv, nc: nc, own: own}, nil
}

func (s *AuthStore) Backend() string { return "nats" }

func (s *AuthStore) Close() error {
	if s == nil {
		return nil
	}
	if s.own && s.nc != nil {
		s.nc.Close()
	}
	return nil
}

// authKVKey 将认证状态 key 编码为 NATS KV 安全 key，与 mute rule 存储解耦。
func authKVKey(key string) string {
	return "auth." + sanitizeKey(key)
}

func (s *AuthStore) put(key, val string) error {
	if s == nil || key == "" {
		return nil
	}
	_, err := s.kv.Put(authKVKey(key), []byte(val))
	return err
}

func (s *AuthStore) get(key string) (string, bool, error) {
	if s == nil || key == "" {
		return "", false, nil
	}
	entry, err := s.kv.Get(authKVKey(key))
	if err != nil {
		if err == nats.ErrKeyNotFound || err == nats.ErrKeyDeleted {
			return "", false, nil
		}
		return "", false, err
	}
	return string(entry.Value()), true, nil
}

func (s *AuthStore) del(key string) error {
	if s == nil || key == "" {
		return nil
	}
	err := s.kv.Delete(authKVKey(key))
	if err == nil || err == nats.ErrKeyNotFound || err == nats.ErrKeyDeleted {
		return nil
	}
	return err
}

// BlacklistToken marks jti revoked. TTL is best-effort via value payload timestamp.
func (s *AuthStore) BlacklistToken(jti string, remaining time.Duration) error {
	if jti == "" || remaining <= 0 {
		return nil
	}
	exp := time.Now().Add(remaining).Unix()
	return s.put("bl."+jti, strconv.FormatInt(exp, 10))
}

// IsBlacklisted 吞掉底层错误按未黑名单返回，保持与 redis.IsBlacklisted 一致的 fail-open 语义；
// 安全敏感场景请使用 IsBlacklistedErr。
func (s *AuthStore) IsBlacklisted(jti string) bool {
	ok, _ := s.IsBlacklistedErr(jti)
	return ok
}

// IsBlacklistedErr 把底层错误上抛，供安全敏感调用方自行决定 fail-open/fail-closed；
// redis.IsBlacklisted 仍保持 fail-open，不改变现有行为。
func (s *AuthStore) IsBlacklistedErr(jti string) (bool, error) {
	if jti == "" {
		return false, nil
	}
	val, ok, err := s.get("bl." + jti)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	exp, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return true, nil
	}
	if time.Now().Unix() > exp {
		_ = s.del("bl." + jti)
		return false, nil
	}
	return true, nil
}

// MarkRefreshFamilyUsed 用 KV Create 实现 SetNX 语义；TTL 由 auth KV bucket（默认 7d）兜底。
func (s *AuthStore) MarkRefreshFamilyUsed(family string, _ time.Duration) (bool, error) {
	if s == nil || family == "" {
		return false, nil
	}
	_, err := s.kv.Create(authKVKey("rf."+family), []byte("1"))
	if errors.Is(err, nats.ErrKeyExists) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *AuthStore) IsRefreshFamilyUsed(family string) (bool, error) {
	if s == nil || family == "" {
		return false, nil
	}
	_, ok, err := s.get("rf." + family)
	return ok, err
}

// RevokeRefreshFamily 覆盖写入已吊销标记，使同 family 的所有 refresh token 失效。
func (s *AuthStore) RevokeRefreshFamily(family string) error {
	if s == nil || family == "" {
		return nil
	}
	return s.put("rf."+family, "revoked")
}

// signingKeyRecord 将 active key 与创建时间合并为单条 KV 记录，
// 避免多实例并发轮换时两个 key 写入交错导致验签不一致。
type signingKeyRecord struct {
	Key       string `json:"key"`
	CreatedAt int64  `json:"created_at"`
}

// signingKeyData 序列化 active key 记录，SetSigningKey 与 UpdateSigningKey 共用。
func signingKeyData(key string, createdAt int64) ([]byte, error) {
	return json.Marshal(signingKeyRecord{Key: key, CreatedAt: createdAt})
}

func (s *AuthStore) GetSigningKey() (string, bool, error) {
	val, ok, err := s.get("jwt.active")
	if err != nil || !ok {
		return "", ok, err
	}
	var rec signingKeyRecord
	if json.Unmarshal([]byte(val), &rec) == nil && rec.Key != "" {
		return rec.Key, true, nil
	}
	// 旧格式：纯 key 字符串
	return val, true, nil
}

// ErrSigningKeyExists 表示已有 active key，拒绝覆盖。
var ErrSigningKeyExists = errors.New("signing key already exists")

// SetSigningKey 使用 KV Create 抢占首启；已存在返回 ErrSigningKeyExists。
func (s *AuthStore) SetSigningKey(key string, createdAtUnix int64) error {
	if key == "" {
		return nil
	}
	data, err := signingKeyData(key, createdAtUnix)
	if err != nil {
		return err
	}
	_, err = s.kv.Create(authKVKey("jwt.active"), data)
	if errors.Is(err, nats.ErrKeyExists) {
		return ErrSigningKeyExists
	}
	return err
}

// UpdateSigningKey 覆盖写入 active key，用于密钥轮换（SetSigningKey 是 CAS 首启语义）。
// 跨实例并发轮换时 active key 采用 last-writer-wins；生产多实例部署应接受该语义或另行加锁。
func (s *AuthStore) UpdateSigningKey(key string, createdAtUnix int64) error {
	if key == "" {
		return nil
	}
	data, err := signingKeyData(key, createdAtUnix)
	if err != nil {
		return err
	}
	return s.put("jwt.active", string(data))
}

func (s *AuthStore) GetCreatedAt() (int64, bool, error) {
	val, ok, err := s.get("jwt.active")
	if err != nil || !ok {
		return 0, false, err
	}
	var rec signingKeyRecord
	if json.Unmarshal([]byte(val), &rec) == nil && rec.Key != "" {
		return rec.CreatedAt, true, nil
	}
	// 旧格式：读取单独创建的 jwt.created_at
	legacy, ok2, err2 := s.get("jwt.created_at")
	if err2 != nil || !ok2 {
		return 0, false, err2
	}
	n, err := strconv.ParseInt(legacy, 10, 64)
	if err != nil {
		return 0, false, nil
	}
	return n, true, nil
}

// AddHistoryKey appends key to history list (newline-separated, capped).
// 跨实例并发轮换时 history 追加是 best-effort 读改写，极端并发下可能丢失中间 key；
// 生产多实例部署应接受该上限或另行加锁。
func (s *AuthStore) AddHistoryKey(key string) error {
	if key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, _, _ := s.get("jwt.history")
	parts := []string{}
	if cur != "" {
		parts = strings.Split(cur, "\n")
	}
	// prepend unique
	out := make([]string, 0, len(parts)+1)
	out = append(out, key)
	for _, p := range parts {
		if p == "" || p == key {
			continue
		}
		out = append(out, p)
		if len(out) >= 32 {
			break
		}
	}
	return s.put("jwt.history", strings.Join(out, "\n"))
}

func (s *AuthStore) HistoryKeys() []string {
	val, ok, err := s.get("jwt.history")
	if err != nil || !ok || val == "" {
		return nil
	}
	parts := strings.Split(val, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
