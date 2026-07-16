package bus

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"GOSpeak/internal/sfu"

	"github.com/nats-io/nats.go"
	goredis "github.com/redis/go-redis/v9"
)

const defaultMuteRuleTTL = 24 * time.Hour

// RedisMuteRuleStore stores Agora (etc.) mute rule ids in Redis with per-key TTL.
type RedisMuteRuleStore struct {
	rdb    *goredis.Client
	prefix string
}

type RedisMuteRuleStoreConfig struct {
	Client *goredis.Client
	Prefix string
}

func OpenRedisMuteRuleStore(cfg RedisMuteRuleStoreConfig) (*RedisMuteRuleStore, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("redis mute rule store: nil client")
	}
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cfg.Client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis mute rule store ping: %w", err)
	}
	return &RedisMuteRuleStore{rdb: cfg.Client, prefix: cfg.Prefix}, nil
}

func (s *RedisMuteRuleStore) Backend() string { return "redis" }

func (s *RedisMuteRuleStore) key(k string) string {
	return s.prefix + ":sfu:mute_rule:" + sanitizeKey(k)
}

func (s *RedisMuteRuleStore) Save(ctx context.Context, key string, ruleID int, ttl time.Duration) error {
	if s == nil || key == "" || ruleID <= 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = defaultMuteRuleTTL
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}
	return s.rdb.Set(ctx, s.key(key), strconv.Itoa(ruleID), ttl).Err()
}

func (s *RedisMuteRuleStore) Get(ctx context.Context, key string) (int, error) {
	if s == nil || key == "" {
		return 0, nil
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}
	val, err := s.rdb.Get(ctx, s.key(key)).Result()
	if err == goredis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	id, err := strconv.Atoi(val)
	if err != nil {
		return 0, nil
	}
	return id, nil
}

func (s *RedisMuteRuleStore) Delete(ctx context.Context, key string) error {
	if s == nil || key == "" {
		return nil
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}
	return s.rdb.Del(ctx, s.key(key)).Err()
}

// NATSMuteRuleStore stores mute rule ids in JetStream KV.
// Bucket TTL is coarse (default 24h); per-key shorter TTL is not available on
// this nats.go version, so callers should still delete on unmute.
type NATSMuteRuleStore struct {
	kv  nats.KeyValue
	nc  *nats.Conn
	own bool
}

type NATSMuteRuleStoreConfig struct {
	URL    string
	Prefix string
	NC     *nats.Conn
	// BucketTTL is the JetStream KV bucket MaxAge. Zero => 24h.
	BucketTTL time.Duration
}

func OpenNATSMuteRuleStore(cfg NATSMuteRuleStoreConfig) (*NATSMuteRuleStore, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}
	if cfg.BucketTTL <= 0 {
		cfg.BucketTTL = defaultMuteRuleTTL
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
			TTL:    cfg.BucketTTL,
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

func (s *NATSMuteRuleStore) Save(_ context.Context, key string, ruleID int, _ time.Duration) error {
	if s == nil || key == "" || ruleID <= 0 {
		return nil
	}
	// JetStream KV key cannot contain '|'; sanitize.
	_, err := s.kv.Put(muteRuleKVKey(key), []byte(strconv.Itoa(ruleID)))
	return err
}

func (s *NATSMuteRuleStore) Get(_ context.Context, key string) (int, error) {
	if s == nil || key == "" {
		return 0, nil
	}
	entry, err := s.kv.Get(muteRuleKVKey(key))
	if err != nil {
		if err == nats.ErrKeyNotFound || err == nats.ErrKeyDeleted {
			return 0, nil
		}
		return 0, err
	}
	id, err := strconv.Atoi(string(entry.Value()))
	if err != nil {
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
// Mode: auto|redis|nats|memory (default auto).
// auto preference: redis → nats → memory.
// Always returns a non-nil store (at least memory).
type ResolveMuteRuleConfig struct {
	Mode   string
	Prefix string
	Redis  *goredis.Client
	NATS   *nats.Conn
}

// ResolveMuteRuleStore opens shared mute-rule store with degradation.
// Result is always non-nil: CachedMuteRuleStore over redis/nats, or pure memory.
func ResolveMuteRuleStore(cfg ResolveMuteRuleConfig) (sfu.MuteRuleStore, string) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		mode = "auto"
	}
	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "gospeak"
	}

	tryRedis := func() (sfu.MuteRuleStore, error) {
		if cfg.Redis == nil {
			return nil, fmt.Errorf("redis client not connected")
		}
		return OpenRedisMuteRuleStore(RedisMuteRuleStoreConfig{
			Client: cfg.Redis,
			Prefix: prefix,
		})
	}
	tryNATS := func() (sfu.MuteRuleStore, error) {
		if cfg.NATS == nil {
			return nil, fmt.Errorf("nats connection not available")
		}
		return OpenNATSMuteRuleStore(NATSMuteRuleStoreConfig{
			Prefix: prefix,
			NC:     cfg.NATS,
		})
	}

	wrap := func(shared sfu.MuteRuleStore) sfu.MuteRuleStore {
		return sfu.NewCachedMuteRuleStore(shared)
	}

	switch mode {
	case "memory", "none":
		return sfu.NewMemoryMuteRuleStore(), "memory"
	case "redis":
		if st, err := tryRedis(); err == nil {
			return wrap(st), "redis"
		} else {
			log.Printf("[MuteRuleStore] redis unavailable: %v; fallback memory", err)
			return sfu.NewMemoryMuteRuleStore(), "memory"
		}
	case "nats":
		if st, err := tryNATS(); err == nil {
			return wrap(st), "nats"
		} else {
			log.Printf("[MuteRuleStore] nats unavailable: %v; fallback memory", err)
			return sfu.NewMemoryMuteRuleStore(), "memory"
		}
	case "auto":
		if st, err := tryRedis(); err == nil {
			return wrap(st), "redis"
		} else {
			log.Printf("[MuteRuleStore] redis unavailable: %v; try nats", err)
		}
		if st, err := tryNATS(); err == nil {
			return wrap(st), "nats"
		} else {
			log.Printf("[MuteRuleStore] nats unavailable: %v; fallback memory", err)
		}
		return sfu.NewMemoryMuteRuleStore(), "memory"
	default:
		log.Printf("[MuteRuleStore] unknown mode %q; fallback memory", mode)
		return sfu.NewMemoryMuteRuleStore(), "memory"
	}
}

// Ensure interface compliance.
var (
	_ sfu.MuteRuleStore = (*RedisMuteRuleStore)(nil)
	_ sfu.MuteRuleStore = (*NATSMuteRuleStore)(nil)
)
