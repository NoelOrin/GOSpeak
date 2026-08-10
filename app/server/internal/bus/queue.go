package bus

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// JobEnvelope is a durable async job published to JetStream.
type JobEnvelope struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	TS      int64           `json:"ts"`
	// TargetNodeID 非空时投递到 {prefix}.jobs.{type}.{nodeID}，
	// 广播消费者仍会收到并由 handler 自行过滤。
	TargetNodeID string `json:"target_node_id,omitempty"`
}

// JobHandler processes one job; return error to Nak for retry.
type JobHandler func(job JobEnvelope) error

// JobQueueConfig opens a work-queue stream for async jobs.
type JobQueueConfig struct {
	URL    string
	Prefix string
	NC     *nats.Conn
	Name   string // consumer/durable name prefix
}

// JobQueue publishes and consumes jobs on {prefix}.jobs.>
type JobQueue struct {
	nc        *nats.Conn
	js        nats.JetStreamContext
	prefix    string
	own       bool
	roleAware bool
	mu        sync.Mutex
	subs      []*nats.Subscription
}

// OpenJobQueue ensures stream {prefix}_jobs and returns a queue handle.
// New streams use LimitsPolicy so role-aware consumers can split chat
// persistence (Agent) from runtime jobs (Worker). Legacy WorkQueue streams
// keep the previous all-instance behavior.
func OpenJobQueue(cfg JobQueueConfig) (*JobQueue, error) {
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
			return nil, fmt.Errorf("job queue: empty URL and nil NC")
		}
		nc, err = nats.Connect(cfg.URL, nats.Name(cfg.Prefix+"-jobs"), nats.Timeout(2*time.Second))
		if err != nil {
			return nil, fmt.Errorf("job queue connect: %w", err)
		}
		own = true
	}
	js, err := nc.JetStream()
	if err != nil {
		if own {
			nc.Close()
		}
		return nil, fmt.Errorf("job queue jetstream: %w", err)
	}
	stream := cfg.Prefix + "_jobs"
	subject := cfg.Prefix + ".jobs.>"
	info, err := js.StreamInfo(stream)
	if err != nil {
		_, err = js.AddStream(&nats.StreamConfig{
			Name:      stream,
			Subjects:  []string{subject},
			Retention: nats.LimitsPolicy,
			Storage:   nats.FileStorage,
			MaxAge:    24 * time.Hour,
		})
		if err != nil {
			if own {
				nc.Close()
			}
			return nil, fmt.Errorf("job queue stream: %w", err)
		}
	}
	roleAware := true
	if info != nil && info.Config.Retention != nats.LimitsPolicy {
		// 旧版 WorkQueue stream 只允许一个 consumer；保持全实例消费兼容，
		// 新部署使用 LimitsPolicy 后可启用 chat/runtime 角色分流。
		roleAware = false
	}
	return &JobQueue{nc: nc, js: js, prefix: cfg.Prefix, own: own, roleAware: roleAware}, nil
}

// RoleAware reports whether the underlying stream supports role-based consumers.
func (q *JobQueue) RoleAware() bool {
	return q.roleAware
}

func (q *JobQueue) subjectFor(jobType string) string {
	return q.prefix + ".jobs." + jobType
}

// Publish enqueues a job. Subject = {prefix}.jobs.{type}
func (q *JobQueue) Publish(ctx context.Context, job JobEnvelope) error {
	_ = ctx
	if job.TS == 0 {
		job.TS = time.Now().UnixMilli()
	}
	if job.Type == "" {
		return fmt.Errorf("job queue: empty type")
	}
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	opts := []nats.PubOpt{}
	if job.ID != "" {
		// 以 job ID 作为 JetStream MsgId，生产端重试/重放时由服务端去重。
		opts = append(opts, nats.MsgId(job.ID))
	}
	subject := q.subjectFor(job.Type)
	if job.TargetNodeID != "" {
		subject += "." + job.TargetNodeID
	}
	_, err = q.js.Publish(subject, b, opts...)
	return err
}

// PublishSRS is a typed helper for SRS stream callbacks.
func (q *JobQueue) PublishSRS(ctx context.Context, action, stream string) error {
	payload, _ := json.Marshal(map[string]string{"action": action, "stream": stream})
	return q.Publish(ctx, JobEnvelope{
		ID:      fmt.Sprintf("srs-%s-%s-%d", action, stream, time.Now().UnixNano()),
		Type:    "srs",
		Payload: payload,
	})
}

// PublishLiveKit enqueues a raw LiveKit webhook body.
func (q *JobQueue) PublishLiveKit(ctx context.Context, raw []byte) error {
	return q.Publish(ctx, JobEnvelope{
		ID:      fmt.Sprintf("lk-%d", time.Now().UnixNano()),
		Type:    "livekit",
		Payload: raw,
	})
}

// PublishSFUCleanup enqueues participant cleanup work.
func (q *JobQueue) PublishSFUCleanup(ctx context.Context, room, identity string, deleteRoom bool) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"room":        room,
		"identity":    identity,
		"delete_room": deleteRoom,
	})
	return q.Publish(ctx, JobEnvelope{
		ID:      fmt.Sprintf("cleanup-%s-%s-%d", room, identity, time.Now().UnixNano()),
		Type:    "sfu_cleanup",
		Payload: payload,
	})
}

// ConsumeChat consumes durable chat persistence jobs. Only Agent/all instances
// should subscribe so user data writes stay on the control plane.
func (q *JobQueue) ConsumeChat(durable string, handler JobHandler) (*nats.Subscription, error) {
	if !q.roleAware {
		return nil, fmt.Errorf("job queue stream is not role-aware; use Consume")
	}
	return q.consumeFiltered(q.prefix+".jobs.chat.>", q.prefix+"-chat-workers", q.prefix+"-chat-worker", handler)
}

// ConsumeRuntime consumes runtime jobs (SRS/LiveKit/SFU cleanup/control commands)
// that must run on data-plane instances.
func (q *JobQueue) ConsumeRuntime(durable string, handler JobHandler) ([]*nats.Subscription, error) {
	if !q.roleAware {
		return nil, fmt.Errorf("job queue stream is not role-aware; use Consume")
	}
	subjects := []string{
		q.prefix + ".jobs.srs",
		q.prefix + ".jobs.livekit",
		q.prefix + ".jobs.sfu_cleanup",
	}
	var subs []*nats.Subscription
	for i, subject := range subjects {
		sub, err := q.consumeFiltered(subject, q.prefix+"-runtime-workers", fmt.Sprintf("%s-runtime-%d", q.prefix, i), handler)
		if err != nil {
			for _, s := range subs {
				_ = s.Unsubscribe()
			}
			return nil, err
		}
		subs = append(subs, sub)
	}
	// 控制命令必须广播到每个 Worker：按实例持久化名订阅通配符，
	// 不使用 queue group，避免 mute/kick/delete 只命中一个节点。
	baseDurable := q.prefix + "-control-" + sanitizeDurable(durable)
	controlSubjects := []struct {
		subject string
		durable string
	}{
		{q.prefix + ".jobs.cluster.control", baseDurable + "-broadcast"},
		{q.prefix + ".jobs.cluster.control.>", baseDurable + "-directed"},
	}
	for _, cs := range controlSubjects {
		controlSub, err := q.consumeBroadcast(cs.subject, cs.durable, handler)
		if err != nil {
			for _, s := range subs {
				_ = s.Unsubscribe()
			}
			return nil, err
		}
		subs = append(subs, controlSub)
	}
	return subs, nil
}

// Consume starts a durable push consumer for all jobs (queue group = workers).
// handler errors Nak; success Ack.
func sanitizeDurable(s string) string {
	// JetStream durable names: A-Z a-z 0-9 - _ / = only; no '.' from hostnames.
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '/', r == '=':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "worker"
	}
	return out
}

func (q *JobQueue) Consume(durable string, handler JobHandler) (*nats.Subscription, error) {
	// All instances share one durable + queue group; NATS load-balances deliveries.
	_ = durable
	durableName := sanitizeDurable(q.prefix + "-worker")
	queueGroup := sanitizeDurable(q.prefix + "-workers")
	return q.consumeFiltered(q.prefix+".jobs.>", queueGroup, durableName, handler)
}

func (q *JobQueue) consumeFiltered(subject, queueGroup, durable string, handler JobHandler) (*nats.Subscription, error) {
	durableName := sanitizeDurable(durable)
	group := sanitizeDurable(queueGroup)
	sub, err := q.js.QueueSubscribe(
		subject,
		group,
		func(msg *nats.Msg) {
			var job JobEnvelope
			if err := json.Unmarshal(msg.Data, &job); err != nil {
				log.Printf("[JobQueue] bad job: %v", err)
				_ = msg.Term()
				return
			}
			if err := handler(job); err != nil {
				log.Printf("[JobQueue] handler type=%s err=%v", job.Type, err)
				_ = msg.Nak()
				return
			}
			_ = msg.Ack()
		},
		nats.Durable(durableName),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(5),
	)
	if err != nil {
		return nil, err
	}
	q.mu.Lock()
	q.subs = append(q.subs, sub)
	q.mu.Unlock()
	return sub, nil
}

// consumeBroadcast 为每个实例订阅同一 subject，每条消息都会投递给每个订阅者。
func (q *JobQueue) consumeBroadcast(subject, durable string, handler JobHandler) (*nats.Subscription, error) {
	durableName := sanitizeDurable(durable)
	sub, err := q.js.Subscribe(
		subject,
		func(msg *nats.Msg) {
			var job JobEnvelope
			if err := json.Unmarshal(msg.Data, &job); err != nil {
				log.Printf("[JobQueue] bad broadcast job: %v", err)
				_ = msg.Term()
				return
			}
			if err := handler(job); err != nil {
				log.Printf("[JobQueue] handler type=%s err=%v", job.Type, err)
				_ = msg.Nak()
				return
			}
			_ = msg.Ack()
		},
		nats.Durable(durableName),
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(5),
	)
	if err != nil {
		return nil, err
	}
	q.mu.Lock()
	q.subs = append(q.subs, sub)
	q.mu.Unlock()
	return sub, nil
}

// Close unsubscribes consumers and optionally closes owned connection.
func (q *JobQueue) Close() error {
	q.mu.Lock()
	for _, s := range q.subs {
		_ = s.Unsubscribe()
	}
	q.subs = nil
	q.mu.Unlock()
	if q.own && q.nc != nil {
		q.nc.Close()
	}
	return nil
}
