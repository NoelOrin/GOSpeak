package bus

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestProbeExternal_OKAndFail(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		es, err := StartEmbeddedServer()
		if err != nil {
			t.Fatal(err)
		}
		defer es.Shutdown()

		if err := ProbeExternal(es.ClientURL()); err != nil {
			t.Fatalf("expected OK, got: %v", err)
		}
	})

	t.Run("Fail", func(t *testing.T) {
		// 127.0.0.1:1 is unlikely to have a NATS server
		err := ProbeExternal("nats://127.0.0.1:1")
		if err == nil {
			t.Fatal("expected error probing unreachable address")
		}
	})
}

func TestInit_EmptyURL_StartsEmbedded(t *testing.T) {
	d := &recordingDeliverer{}
	nb, embed, err := Init(InitConfig{
		InstanceID:    "test-empty",
		SubjectPrefix: "gospeak",
		NATSURL:       "",
		Deliverer:     d,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer nb.Close()
	if embed == nil {
		t.Fatal("expected non-nil EmbeddedServer")
	}
	defer embed.Shutdown()

	if nb.Mode() != "embedded" {
		t.Fatalf("Mode = %q, want embedded", nb.Mode())
	}
	if !nb.IsConnected() {
		t.Fatal("expected connected")
	}
	stats := GetStats(nb)
	if stats.FallbackFromExternal {
		t.Fatal("FallbackFromExternal should be false for direct embedded init")
	}
}

func TestInit_ExternalURL_Available_UsesExternalNoEmbed(t *testing.T) {
	es, err := StartEmbeddedServer()
	if err != nil {
		t.Fatal(err)
	}
	defer es.Shutdown()

	d := &recordingDeliverer{}
	nb, embed, err := Init(InitConfig{
		InstanceID:    "test-external-ok",
		SubjectPrefix: "gospeak",
		NATSURL:       es.ClientURL(),
		Deliverer:     d,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer nb.Close()
	if embed != nil {
		embed.Shutdown()
		// We don't own this embed; the external path should not start one
		t.Fatal("expected nil EmbeddedServer for external mode")
	}

	if nb.Mode() != "external" {
		t.Fatalf("Mode = %q, want external", nb.Mode())
	}
	if !nb.IsConnected() {
		t.Fatal("expected connected")
	}
	stats := GetStats(nb)
	if stats.FallbackFromExternal {
		t.Fatal("FallbackFromExternal should be false for clean external init")
	}
}

func TestInit_ExternalURL_Unavailable_FallsBackEmbedded(t *testing.T) {
	// Capture log output to verify the fallback warning
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil) // restore after test

	d := &recordingDeliverer{}
	nb, embed, err := Init(InitConfig{
		InstanceID:    "test-fallback",
		SubjectPrefix: "gospeak",
		NATSURL:       "nats://127.0.0.1:1", // nothing listening here
		Deliverer:     d,
	})
	if err != nil {
		t.Fatalf("Init should not fail on fallback; got: %v", err)
	}
	defer nb.Close()
	if embed == nil {
		t.Fatal("expected non-nil EmbeddedServer (fallback)")
	}
	defer embed.Shutdown()

	if nb.Mode() != "embedded" {
		t.Fatalf("Mode = %q, want embedded (fallback)", nb.Mode())
	}
	if !nb.IsConnected() {
		t.Fatal("expected connected")
	}

	stats := GetStats(nb)
	if !stats.FallbackFromExternal {
		t.Fatal("FallbackFromExternal should be true after fallback")
	}

	logMsg := buf.String()
	if !strings.Contains(logMsg, "probe") {
		t.Fatalf("fallback warning should mention 'probe', got: %q", logMsg)
	}
}

func TestInit_ExternalBadURL_MessageContainsProbe(t *testing.T) {
	// A syntactically bad URL leads to a probe error with "probe" in it.
	err := ProbeExternal("://not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for bad URL")
	}
	if !strings.Contains(err.Error(), "probe") {
		t.Fatalf("ProbeExternal error should mention 'probe', got: %q", err.Error())
	}

	// Init with a bad URL should fall back to embedded.
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	d := &recordingDeliverer{}
	nb, embed, initErr := Init(InitConfig{
		InstanceID:    "test-badurl",
		SubjectPrefix: "gospeak",
		NATSURL:       "nats://127.0.0.1:1",
		Deliverer:     d,
	})
	if initErr != nil {
		t.Fatalf("Init should not fail on fallback; got: %v", initErr)
	}
	defer nb.Close()
	if embed == nil {
		t.Fatal("expected non-nil EmbeddedServer (fallback from bad url)")
	}
	defer embed.Shutdown()

	stats := GetStats(nb)
	if !stats.FallbackFromExternal {
		t.Fatal("FallbackFromExternal should be true after fallback from bad URL")
	}

	logMsg := buf.String()
	if !strings.Contains(logMsg, "probe") {
		t.Fatalf("fallback warning should mention 'probe', got: %q", logMsg)
	}
}
