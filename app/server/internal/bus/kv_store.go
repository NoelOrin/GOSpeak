package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// MemberRecord is one online member in a room membership snapshot.
type MemberRecord struct {
	Room        string `json:"room"`
	Identity    string `json:"identity"`
	SocketHint  string `json:"socket_hint,omitempty"`
	InstanceID  string `json:"instance_id"`
	Stream      string `json:"stream,omitempty"`
	MicMuted    bool   `json:"mic_muted"`
	Speaking    bool   `json:"speaking"`
	UpdatedAtMS int64  `json:"updated_at_ms"`
}

// RoomMembersSnapshot is the KV value for a room's membership view.
type RoomMembersSnapshot struct {
	Room      string         `json:"room"`
	Members   []MemberRecord `json:"members"`
	UpdatedAt int64          `json:"updated_at_ms"`
}

// StateStoreConfig opens JetStream KV buckets for membership/stream state.
type StateStoreConfig struct {
	URL    string
	Prefix string
	NC     *nats.Conn // optional: reuse existing connection
}

// StateStore wraps membership + stream JetStream KV buckets.
type StateStore struct {
	nc   *nats.Conn
	js   nats.JetStreamContext
	mem  nats.KeyValue
	strm nats.KeyValue
	own  bool
}

// OpenStateStore creates/opens membership and stream KV buckets.
func OpenStateStore(cfg StateStoreConfig) (*StateStore, error) {
	if cfg.Prefix == "" {
		cfg.Prefix = "gospeak"
	}

	var nc *nats.Conn
	var err error
	own := false
	if cfg.NC != nil {
		nc = cfg.NC
	} else {
		if strings.TrimSpace(cfg.URL) == "" {
			return nil, fmt.Errorf("state store: empty URL and nil NC")
		}
		nc, err = nats.Connect(cfg.URL, nats.Name(cfg.Prefix+"-state"), nats.Timeout(2*time.Second))
		if err != nil {
			return nil, fmt.Errorf("state store connect: %w", err)
		}
		own = true
	}

	js, err := nc.JetStream()
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("state store jetstream: %w", err)
	}

	mem, err := openOrCreateKV(js, cfg.Prefix+"_membership")
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("membership kv: %w", err)
	}
	strm, err := openOrCreateKV(js, cfg.Prefix+"_stream")
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("stream kv: %w", err)
	}

	return &StateStore{nc: nc, js: js, mem: mem, strm: strm, own: own}, nil
}

func openOrCreateKV(js nats.JetStreamContext, bucket string) (nats.KeyValue, error) {
	kv, err := js.KeyValue(bucket)
	if err == nil {
		return kv, nil
	}
	return js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: bucket,
		TTL:    24 * time.Hour,
	})
}

func (s *StateStore) PutRoomMembers(ctx context.Context, snap RoomMembersSnapshot) error {
	_ = ctx
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	_, err = s.mem.Put("room."+sanitizeKey(snap.Room), b)
	return err
}

func (s *StateStore) GetRoomMembers(ctx context.Context, room string) (RoomMembersSnapshot, error) {
	_ = ctx
	entry, err := s.mem.Get("room." + sanitizeKey(room))
	if err != nil {
		return RoomMembersSnapshot{}, err
	}
	var snap RoomMembersSnapshot
	if err := json.Unmarshal(entry.Value(), &snap); err != nil {
		return RoomMembersSnapshot{}, err
	}
	if snap.Members == nil {
		snap.Members = []MemberRecord{}
	}
	return snap, nil
}

func (s *StateStore) DeleteRoomMembers(ctx context.Context, room string) error {
	_ = ctx
	return s.mem.Delete("room." + sanitizeKey(room))
}

func (s *StateStore) PutStream(ctx context.Context, stream, room, identity string) error {
	_ = ctx
	b, err := json.Marshal(map[string]string{
		"room":     room,
		"identity": identity,
	})
	if err != nil {
		return err
	}
	_, err = s.strm.Put("stream."+sanitizeKey(stream), b)
	return err
}

func (s *StateStore) GetStream(ctx context.Context, stream string) (room, identity string, err error) {
	_ = ctx
	entry, err := s.strm.Get("stream." + sanitizeKey(stream))
	if err != nil {
		return "", "", err
	}
	var m map[string]string
	if err := json.Unmarshal(entry.Value(), &m); err != nil {
		return "", "", err
	}
	return m["room"], m["identity"], nil
}

func (s *StateStore) DeleteStream(ctx context.Context, stream string) error {
	_ = ctx
	return s.strm.Delete("stream." + sanitizeKey(stream))
}


// ListRoomNames returns room names currently present in membership KV.
func (s *StateStore) ListRoomNames(ctx context.Context) ([]string, error) {
	_ = ctx
	keys, err := s.mem.Keys()
	if err != nil {
		// empty bucket surfaces as ErrNoKeysFound on some nats versions
		if err == nats.ErrNoKeysFound {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.HasPrefix(k, "room.") {
			out = append(out, strings.TrimPrefix(k, "room."))
		}
	}
	return out, nil
}

// Close closes the underlying connection only when OpenStateStore created it.
func (s *StateStore) Close() error {
	if s == nil {
		return nil
	}
	if s.own && s.nc != nil {
		s.nc.Close()
	}
	return nil
}

func sanitizeKey(s string) string {
	return strings.ReplaceAll(s, " ", "_")
}
