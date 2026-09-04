package plugin

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
)

type mockPlugin struct {
	name    string
	inited  bool
	started bool
	stopped bool
}

func (m *mockPlugin) Meta() Meta {
	return Meta{Name: m.name, DisplayName: m.name, Version: "0.0.1", Kind: KindBuiltin}
}
func (m *mockPlugin) Init(host Host) error {
	m.inited = true
	host.RegisterHTTP(func(r *gin.RouterGroup) {})
	return nil
}
func (m *mockPlugin) Start(ctx context.Context) error {
	m.started = true
	return nil
}
func (m *mockPlugin) Stop(ctx context.Context) error {
	m.stopped = true
	return nil
}

func TestRegistry_RegisterGetNames(t *testing.T) {
	host := &HostImpl{
		routeFns:    make(map[string][]func(*gin.RouterGroup)),
		sideServers: make(map[string]*sideServer),
	}
	reg := NewRegistry(host)
	p := &mockPlugin{name: "demo"}
	if err := reg.Register(p); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(p); err == nil {
		t.Fatal("expected duplicate error")
	}
	got, ok := reg.Get("demo")
	if !ok || got.Meta().Name != "demo" {
		t.Fatal("get failed")
	}
	if names := reg.Names(); len(names) != 1 || names[0] != "demo" {
		t.Fatalf("names=%v", names)
	}
}

func TestRegistry_InitStartStop(t *testing.T) {
	host := &HostImpl{
		routeFns:    make(map[string][]func(*gin.RouterGroup)),
		sideServers: make(map[string]*sideServer),
	}
	// stub LoadConfig via SaveConfig store not needed: LoadConfig without repo returns error path.
	// Provide a tiny fake by setting repo nil and patching - Host.LoadConfig needs repo.
	// Use StartOne directly after manual Init to avoid LoadConfig.
	reg := NewRegistry(host)
	p := &mockPlugin{name: "demo"}
	if err := reg.Register(p); err != nil {
		t.Fatal(err)
	}
	host.WithPlugin("demo")
	if err := p.Init(host); err != nil {
		t.Fatal(err)
	}
	if !p.inited {
		t.Fatal("not inited")
	}
	if err := reg.StartOne(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if !p.started {
		t.Fatal("not started")
	}
	if err := reg.StopOne(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if !p.stopped {
		t.Fatal("not stopped")
	}
}
