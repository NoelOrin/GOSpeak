package bus

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestJobQueue_PublishConsume(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	q, err := OpenJobQueue(JobQueueConfig{URL: es.ClientURL(), Prefix: "gospeak_qtest"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })

	done := make(chan JobEnvelope, 1)
	if _, err := q.Consume("worker-1", func(job JobEnvelope) error {
		done <- job
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	payload, _ := json.Marshal(map[string]string{"room": "r", "identity": "u"})
	if err := q.Publish(context.Background(), JobEnvelope{
		ID: "1", Type: "sfu_cleanup", Payload: payload, TS: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case job := <-done:
		if job.Type != "sfu_cleanup" {
			t.Fatalf("type %s", job.Type)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for job")
	}
}

func TestJobQueue_ConsumeSanitizesDurable(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)
	q, err := OpenJobQueue(JobQueueConfig{URL: es.ClientURL(), Prefix: "gospeak_qtest3"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })
	// hostname-like instance id with dots must be accepted
	if _, err := q.Consume("gospeak-host.local-123", func(job JobEnvelope) error { return nil }); err != nil {
		t.Fatalf("Consume with dotted durable: %v", err)
	}
}

func TestJobQueue_TwoConsumersSameStream(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(es.Shutdown)

	q1, err := OpenJobQueue(JobQueueConfig{URL: es.ClientURL(), Prefix: "gospeak_qdual"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q1.Close() })
	q2, err := OpenJobQueue(JobQueueConfig{URL: es.ClientURL(), Prefix: "gospeak_qdual"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q2.Close() })

	done := make(chan string, 2)
	handler := func(id string) JobHandler {
		return func(job JobEnvelope) error {
			done <- id
			return nil
		}
	}
	if _, err := q1.Consume("inst-a.host-1", handler("a")); err != nil {
		t.Fatalf("consume1: %v", err)
	}
	if _, err := q2.Consume("inst-b.host-2", handler("b")); err != nil {
		t.Fatalf("consume2: %v", err)
	}
	if err := q1.PublishLiveKit(context.Background(), []byte(`{"event":"test"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		// one of the workers handled it (load balanced)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout: neither consumer got the job")
	}
}
