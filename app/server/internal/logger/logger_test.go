package logger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestInitJSONAndLevel(t *testing.T) {
	var buf bytes.Buffer
	if err := Init(Options{Level: "debug", Format: "json", Output: "stdout", Production: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	L().SetOutput(&buf)
	L().SetFormatter(&logrus.JSONFormatter{})

	WithComponent("Test").WithField("k", "v").Info("hello")
	line := buf.String()
	if !strings.Contains(line, `"component":"Test"`) && !strings.Contains(line, `"component": "Test"`) {
		// logrus JSON may or may not have spaces depending version; parse instead
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &m); err != nil {
		t.Fatalf("json: %v line=%q", err, line)
	}
	if m["msg"] != "hello" {
		t.Fatalf("msg=%v", m["msg"])
	}
	if m["component"] != "Test" {
		t.Fatalf("component=%v", m["component"])
	}
	if m["k"] != "v" {
		t.Fatalf("k=%v", m["k"])
	}
}

func TestInitRejectsBadLevel(t *testing.T) {
	if err := Init(Options{Level: "nope"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestInitFileOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := Init(Options{Level: "info", Format: "text", Output: "file", FilePath: path}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	Info("file-line")
	_ = Close()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "file-line") {
		t.Fatalf("log file content=%q", b)
	}
}

func TestNormalizeDefaults(t *testing.T) {
	dev := normalizeOptions(Options{Production: false})
	if dev.Level != "debug" || dev.Format != "text" {
		t.Fatalf("dev defaults: %+v", dev)
	}
	prod := normalizeOptions(Options{Production: true})
	if prod.Level != "info" || prod.Format != "json" {
		t.Fatalf("prod defaults: %+v", prod)
	}
}
