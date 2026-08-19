package bus

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"GOSpeak/internal/sfu"

	"github.com/nats-io/nats.go"
)

// defaultMuteRuleTTL is the JetStream KV bucket MaxAge used when
// NATSMuteRuleStoreConfig.BucketTTL is nil. It is a physical upper bound, not
// a substitute for per-key TTLs: values carry their own expiry (enforced
// lazily on Get), and a key with a shorter per-key TTL expires before this
// cap. Set BucketTTL=0 or a longer value to keep permanent rules (ttl<=0)
// alive.
const defaultMuteRuleTTL = 24 * time.Hour

// secondsToMillisCutoff separates legacy unix-seconds values (pre-5138) from
// millisecond timestamps written by the current format.
const secondsToMillisCutoff = int64(100_000_000_000)

// NATSMuteRuleStore stores mute rule ids in JetStream KV.
// Values are "ruleID" (permanent until unmute) or "ruleID:unixMilli"
// (per-key TTL); expiry is enforced lazily on read, and unmute removes
// entries with Delete. The bucket MaxAge defaults to 24h as a physical cap.
type NATSMuteRuleStore struct {
	kv  nats.KeyValue
	nc  *nats.Conn
	own bool
}

type NATSMuteRuleStoreConfig struct {
	URL    string
	Prefix string
	NC     *nats.Conn
	// BucketTTL overrides the JetStream KV bucket MaxAge, the physical upper
	// bound for every entry. nil uses the default 24h; a pointer to 0 disables
	// bucket-level expiry, and longer values support permanent mutes. Per-key
	// TTLs encoded in values still expire entries before this cap. An
	// already-created bucket keeps the MaxAge it was created with, so
	// migrating requires recreating the bucket.
	BucketTTL *time.Duration
}

// OpenNATSMuteRuleStore opens (or reuses) the mute-rule KV bucket. Bucket
// MaxAge defaults to 24h as a physical cap; NATSMuteRuleStoreConfig.BucketTTL
// overrides it (a pointer to 0 disables bucket-level expiry). Per-key TTLs
// encoded in values are enforced lazily on Get, so an entry expires at its own
// deadline even when the bucket survives. Existing buckets keep the MaxAge
// they were created with, so changing the default requires recreating the
// bucket.
func OpenNATSMuteRuleStore(cfg NATSMuteRuleStoreConfig) (*NATSMuteRuleStore, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	bucketTTL := defaultMuteRuleTTL
	if cfg.BucketTTL != nil {
		bucketTTL = *cfg.BucketTTL
		if bucketTTL < 0 {
			bucketTTL = 0
		}
	}

	var nc *nats.Conn
	var err error
	own := false
	if cfg.NC != nil {
		nc = cfg.NC
	} else {
		if strings.TrimSpace(cfg.URL) == "" {
			return nil, fmt.Errorf("nats mute rule store: empty URL and nil NC")
		}
		nc, err = nats.Connect(cfg.URL, nats.Name(cfg.Prefix+"-mute-rule"), nats.Timeout(2*time.Second))
		if err != nil {
			return nil, fmt.Errorf("nats mute rule store connect: %w", err)
		}
		own = true
	}

	js, err := nc.JetStream()
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("nats mute rule store jetstream: %w", err)
	}

	bucket := cfg.Prefix + "_sfu_mute_rule"
	kv, err := js.KeyValue(bucket)
	if err != nil {
		kv, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket: bucket,
			TTL:    bucketTTL,
		})
		if err != nil {
			if own {
				nc.Close()
			}
			return nil, fmt.Errorf("nats mute rule store kv: %w", err)
		}
	}
	return &NATSMuteRuleStore{kv: kv, nc: nc, own: own}, nil
}

func (s *NATSMuteRuleStore) Backend() string { return "nats" }

// muteRuleKVKey maps logical keys (may contain '|') to NATS KV-safe keys.
func muteRuleKVKey(key string) string {
	// NATS KV forbids spaces and some tokens; also avoid '|' ambiguity.
	k := strings.ReplaceAll(key, " ", "_")
	k = strings.ReplaceAll(k, "|", ".")
	k = strings.ReplaceAll(k, "/", ".")
	return k
}

func (s *NATSMuteRuleStore) Close() error {
	if s == nil {
		return nil
	}
	if s.own && s.nc != nil {
		s.nc.Close()
	}
	return nil
}

func (s *NATSMuteRuleStore) Save(_ context.Context, key string, ruleID int, ttl time.Duration) error {
	if s == nil || key == "" || ruleID <= 0 {
		return nil
	}
	value := strconv.Itoa(ruleID)
	if ttl > 0 {
		value = fmt.Sprintf("%d:%d", ruleID, time.Now().Add(ttl).UnixMilli())
	}
	_, err := s.kv.Put(muteRuleKVKey(key), []byte(value))
	return err
}

// parseMuteRuleValue 解析 value "ruleID" 或 "ruleID:expiresAt"。
// 无冒号表示永久规则（无过期时间）；有冒号时 expiresAt 必须为正数，
// 否则视为损坏值返回错误，调用方按 miss 处理，不与永久规则混淆。
// expiresAt 统一归一化为毫秒：新格式直接使用毫秒时间戳，旧格式秒值按秒处理。
func parseMuteRuleValue(value string) (ruleID int, expiresAt int64, err error) {
	if i := strings.IndexByte(value, ':'); i >= 0 {
		ruleID, err = strconv.Atoi(value[:i])
		if err != nil {
			return 0, 0, err
		}
		if ruleID <= 0 {
			return 0, 0, fmt.Errorf("invalid mute rule id %q", value[:i])
		}
		expiresAt, err = strconv.ParseInt(value[i+1:], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		if expiresAt <= 0 {
			return 0, 0, fmt.Errorf("invalid mute rule expiry %q", value[i+1:])
		}
		if expiresAt > 0 && expiresAt < secondsToMillisCutoff {
			expiresAt *= 1000
		}
		return ruleID, expiresAt, nil
	}
	ruleID, err = strconv.Atoi(value)
	if err != nil {
		return 0, 0, err
	}
	if ruleID <= 0 {
		return 0, 0, fmt.Errorf("invalid mute rule id %q", value)
	}
	return ruleID, 0, nil
}

func (s *NATSMuteRuleStore) Get(_ context.Context, key string) (int, error) {
	if s == nil || key == "" {
		return 0, nil
	}
	kvKey := muteRuleKVKey(key)
	entry, err := s.kv.Get(kvKey)
	if err != nil {
		if err == nats.ErrKeyNotFound || err == nats.ErrKeyDeleted {
			return 0, nil
		}
		return 0, err
	}
	id, expiresAt, err := parseMuteRuleValue(string(entry.Value()))
	if err != nil {
		return 0, nil
	}
	if expiresAt > 0 && time.Now().After(time.UnixMilli(expiresAt)) {
		// 过期值按 miss 返回，entry 保留到后续覆盖或显式 Delete；
		// 这里不做惰性删除，避免与并发 Save 交错时误删新规则。
		return 0, nil
	}
	return id, nil
}

func (s *NATSMuteRuleStore) Delete(_ context.Context, key string) error {
	if s == nil || key == "" {
		return nil
	}
	err := s.kv.Delete(muteRuleKVKey(key))
	if err == nil || err == nats.ErrKeyNotFound || err == nats.ErrKeyDeleted {
		return nil
	}
	return err
}

// ResolveMuteRuleConfig selects mute-rule backend.
// Mode: auto|nats (default auto). In-memory / none fallbacks are forbidden:
// the process always runs with a NATS connection (embedded or external,
// guaranteed by bus.Init), so a missing NATS connection is treated as a
// fatal startup error rather than a silent degradation.
// 注意：CachedMuteRuleStore 保留 30s L1 以降低 KV 压力；SRS 场景（ruleID 恒为 1）
// 必须使用 GetFresh 绕过 L1，否则 NATS 抖动会放大解禁延迟，监控需观察 miss/false-hit。
type ResolveMuteRuleConfig struct {
	Mode      string
	Prefix    string
	NATS      *nats.Conn
	BucketTTL *time.Duration
}

// ResolveMuteRuleStore opens the shared mute-rule store backed by NATS KV.
// It never degrades to an in-memory store: when NATS is unavailable it returns
// an error so the caller can fail fast during startup.
func ResolveMuteRuleStore(cfg ResolveMuteRuleConfig) (sfu.MuteRuleStore, string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "auto"
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "gospeak"
	}
	if mode != "auto" && mode != "nats" {
		return nil, "", fmt.Errorf("mute rule store: unsupported mode %q (want auto|nats)", mode)
	}
	if cfg.NATS == nil {
		return nil, "", fmt.Errorf("mute rule store: nats connection required (embedded or external URL)")
	}
	shared, err := OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
		Prefix:    prefix,
		NC:        cfg.NATS,
		BucketTTL: cfg.BucketTTL,
	})
	if err != nil {
		return nil, "", fmt.Errorf("mute rule store nats: %w", err)
	}
	return sfu.NewCachedMuteRuleStore(shared), "nats", nil
}

// Ensure interface compliance.
var (
	_ sfu.MuteRuleStore = (*NATSMuteRuleStore)(nil)
)
