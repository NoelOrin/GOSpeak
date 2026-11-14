# 事件协议单一来源 P1 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 事件名 + ACK 约定收敛到 `internal/protocol` 单一事实源，Go 侧生成前端 TS 产物，CI 拦漂移。规格见 `docs/superpowers/specs/2026-11-14-event-protocol-single-source-design.md`。

**架构：** 新增 `internal/protocol` 包（事件常量 + Catalog 目录 + 4 个既有 DTO），`cmd/gen-protocol` 用 go/ast 解析生成 `app/web/src/protocol/generated/{events,payloads,envelope}.ts`；ACK 统一为「handler 返回纯 data、错误走 err 路径」；防漂移三件套（golden 测试、内存比对测试、CI/pre-push gen-diff）。

**技术栈：** Go（go/parser、go/ast、encoding/json）、TypeScript、pnpm script、lefthook、GitHub Actions。

**P1 范围界定（对应规格 §4）：**
- Catalog 覆盖**全部事件名**；payload 仅挂现有 4 个 DTO（`RoomRequest`/`MemberInfo`/`RoomInfo`/`MuteInfo`），其余事件 `Payload: ""` 留给 P2
- `emitAck` 返回类型在 P1 仅做事件名约束（`Promise<unknown>`），ack-data 类型收窄等 P2 补 ack payload struct 后开启
- 与规格的唯一偏差：`protocol/envelope.go` 不建——ACK 信封类型（`ws.ACK`）已存在于 `internal/ws/types.go`，Go 侧无需重复；`envelope.ts` 由生成器模板产出

**关键已核实事实（执行者不必重查）：**
- `signal/events.go` 43 常量；`web/src/socket/events.ts` 40 键，缺 `connection`/`disconnect`（服务端内部事件），多 `ANNOUNCEMENT_NEW`（生产端常量在 `service/announcement_service.go:19` `EventAnnouncementNew`）
- 事件注册全集：`signal/hub.go:231-250 SetupFanout`（16 个 handler，其中 9 个 Ack）
- ACK 旁路：`message_bridge.go` 全部 5 个 `OnMessage*` + `private_bridge.go OnPrivateMessageSend` 把错误塞进成功 ACK 的 data（`err` 恒 nil）；`hub_room_events.go:64-67,92-95` 返回 `{"ok":true,"room":...}`；辅助函数 `resolveMessageRoom`/`checkMessagePerm`/`messageErrAck`/`marshalPolicyAck` 返回内嵌错误的 JSON 串
- `marshalAck` 定义在 `signal/hub_room_join.go:656`，`parseJSON` 在 `signal/hub.go:352`
- 前端 ACK 消费：`wsClient.ts:186-190` 已把 `msg.error` 走 reject、`tryParse(msg.data)` 走 resolve（`tryParse("")` 返回 `""`，空串 data 会 resolve 成字符串——所以 Go 侧成功时统一 `return "", nil`，`ACK.Data` 的 `omitempty` 会直接省略 data 字段）；`chatStore.ts:445,510,897` 三处 `resp?.error` 是仅有的 ACK-data 错误消费点；`chatStore.ts:207 handleError` 与 `socketEvents.ts:72 room.error` 是**推送事件**的 data 内嵌错误，属 P2 payload 范围，P1 不动
- 防线挂点：根 `package.json` scripts（~26 行）、`lefthook.yml` pre-push 块、`.github/workflows/ci.yml` 中跑 `go test` 的 server job

---

### 任务 0：准备隔离工作区

**文件：** 无（git 操作）

- [ ] **步骤 1：创建 worktree 分支**（仓库存在并发会话，勿在 release 上直接改）

```bash
git worktree add .worktrees/event-protocol-p1 -b feature/event-protocol-p1 release
cd .worktrees/event-protocol-p1
```

- [ ] **步骤 2：确认基线绿**

```bash
cd app/server && go test ./... && cd ../.. && pnpm --filter @gospeak/web test
```

预期：全部 PASS。有红先停下报告，不带病前进。

---

### 任务 1：`internal/protocol` 包 + signal 别名切换

**文件：**
- 创建：`app/server/internal/protocol/events.go`
- 创建：`app/server/internal/protocol/payloads.go`
- 创建：`app/server/internal/protocol/catalog_test.go`
- 修改：`app/server/internal/signal/events.go`（整文件替换为别名）
- 修改：`app/server/internal/signal/types.go`（整文件替换为别名）
- 修改：`app/server/internal/service/announcement_service.go:19`（常量改引 protocol）

- [ ] **步骤 1：编写失败的 Catalog 完整性测试**

```go
package protocol

import "testing"

func TestCatalogIntegrity(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Catalog {
		if e.Name == "" {
			t.Fatal("catalog entry with empty Name")
		}
		if seen[e.Name] {
			t.Fatalf("duplicate event name: %s", e.Name)
		}
		seen[e.Name] = true
		switch e.Dir {
		case DirClientToServer, DirServerToClient, DirBoth, DirInternal:
		default:
			t.Fatalf("event %s: invalid dir %q", e.Name, e.Dir)
		}
		if e.Dir == DirClientToServer && e.Ack && e.Payload != "" &&
			!payloadStructs[e.Payload] {
			t.Fatalf("event %s: payload %q not defined in package", e.Name, e.Payload)
		}
	}
	// 事件名必须等于常量值（防 Catalog 手抄漂移）
	for _, e := range Catalog {
		if v, ok := eventConsts[e.Name]; ok && v != e.Name {
			t.Fatalf("event %s: constant value mismatch %q", e.Name, v)
		}
	}
}
```

- [ ] **步骤 2：运行确认失败**

```bash
cd app/server && go test ./internal/protocol/
```

预期：FAIL（包不存在，编译错误）。

- [ ] **步骤 3：实现 protocol 包**

`internal/protocol/events.go`：

```go
// Package protocol 是 WS 事件协议的单一事实源：事件名、方向、ACK 需求、
// payload struct 全部在此定义。前端 TS 产物由 cmd/gen-protocol 从本包生成，
// 禁止在任何其他包手工书写 wire 事件名字符串。
package protocol

type Direction string

const (
	DirClientToServer Direction = "c2s"
	DirServerToClient Direction = "s2c"
	DirBoth           Direction = "both"
	DirInternal       Direction = "internal"
)

const (
	EventConnect    = "connection"
	EventDisconnect = "disconnect"

	EventRoomCreate  = "room:create"
	EventRoomJoin    = "room:join"
	EventRoomLeave   = "room:leave"
	EventRoomList    = "room:list"
	EventRoomJoinSFU = "room:join:sfu"

	EventRoomCreated    = "room:created"
	EventRoomLeft       = "room:left"
	EventRoomUpdated    = "room:updated"
	EventMemberJoined   = "member:joined"
	EventMemberLeft     = "member:left"
	EventMemberUpdated  = "member:updated"
	EventRoomListResult = "room:list:result"

	EventMemberMicState = "member:mic-state"
	EventRoomKick       = "room:kick"
	EventRoomKicked     = "room:kicked"

	EventMemberSpeaking     = "member:speaking"
	EventRoomActiveSpeakers = "room:active-speakers"

	// 文字房「正在输入」：客户端上报 typing，服务端按房间广播（含上报者
	// identity，前端自行过滤自己）。不落库、不保证送达，纯 UI 提示。
	EventTyping = "typing"

	EventUserMuted   = "user:muted"
	EventUserUnmuted = "user:unmuted"

	// 成员级禁言事件（订阅端静音用）：所有客户端按 identity 静音/恢复远端 track。
	EventMemberMuted   = "member:muted"
	EventMemberUnmuted = "member:unmuted"

	EventSFUProviderChanged = "sfu:provider-changed"

	EventMessageSend    = "message:send"
	EventMessageEdit    = "message:edit"
	EventMessageDelete  = "message:delete"
	EventMessageReact   = "message:react"
	EventMessageUnreact = "message:unreact"

	EventMessageCreated  = "message:created"
	EventMessageUpdated  = "message:updated"
	EventMessageDeleted  = "message:deleted"
	EventMessageReaction = "message:reaction"
	EventMessageAck      = "message:ack"
	EventMessageError    = "message:error"

	EventPrivateSend = "private:send"
	EventPrivateNew  = "private:new"

	// 系统公告（服务端推送）：平台公告走 namespace 广播，域公告定向投递到 __user 房间。
	EventAnnouncementNew = "announcement:new"
)

// Event 是目录中的一个事件条目。
type Event struct {
	Name    string    // wire 事件名
	Dir     Direction // 方向；DirInternal 不产出前端 TS
	Ack     bool      // 上行是否需要 ACK（仅 DirClientToServer/DirBoth 有意义）
	Payload string    // payloads.go 中的 struct 名；"" 表示 P1 未挂 payload（P2 补齐）
}

// payloadStructs 供测试校验 Payload 引用；每新增 payload struct 同步登记。
var payloadStructs = map[string]bool{
	"RoomRequest": true,
	"MemberInfo":  true,
	"RoomInfo":    true,
	"MuteInfo":    true,
}

// eventConsts 供测试校验 Catalog 与常量一致；登记全部事件名常量。
var eventConsts = map[string]string{
	EventConnect: EventConnect, EventDisconnect: EventDisconnect,
	EventRoomCreate: EventRoomCreate, EventRoomJoin: EventRoomJoin,
	EventRoomLeave: EventRoomLeave, EventRoomList: EventRoomList,
	EventRoomJoinSFU: EventRoomJoinSFU, EventRoomCreated: EventRoomCreated,
	EventRoomLeft: EventRoomLeft, EventRoomUpdated: EventRoomUpdated,
	EventMemberJoined: EventMemberJoined, EventMemberLeft: EventMemberLeft,
	EventMemberUpdated: EventMemberUpdated, EventRoomListResult: EventRoomListResult,
	EventMemberMicState: EventMemberMicState, EventRoomKick: EventRoomKick,
	EventRoomKicked: EventRoomKicked, EventMemberSpeaking: EventMemberSpeaking,
	EventRoomActiveSpeakers: EventRoomActiveSpeakers, EventTyping: EventTyping,
	EventUserMuted: EventUserMuted, EventUserUnmuted: EventUserUnmuted,
	EventMemberMuted: EventMemberMuted, EventMemberUnmuted: EventMemberUnmuted,
	EventSFUProviderChanged: EventSFUProviderChanged,
	EventMessageSend: EventMessageSend, EventMessageEdit: EventMessageEdit,
	EventMessageDelete: EventMessageDelete, EventMessageReact: EventMessageReact,
	EventMessageUnreact: EventMessageUnreact, EventMessageCreated: EventMessageCreated,
	EventMessageUpdated: EventMessageUpdated, EventMessageDeleted: EventMessageDeleted,
	EventMessageReaction: EventMessageReaction, EventMessageAck: EventMessageAck,
	EventMessageError: EventMessageError, EventPrivateSend: EventPrivateSend,
	EventPrivateNew: EventPrivateNew, EventAnnouncementNew: EventAnnouncementNew,
}

// Catalog 是全部事件的权威目录。Ack 标志与 signal/hub.go SetupFanout 的
// Handle/HandleAck 注册一一对应；P2 逐事件补 Payload。
var Catalog = []Event{
	{Name: EventConnect, Dir: DirInternal},
	{Name: EventDisconnect, Dir: DirInternal},

	{Name: EventRoomCreate, Dir: DirClientToServer},
	{Name: EventRoomJoin, Dir: DirClientToServer, Ack: true, Payload: "RoomRequest"},
	{Name: EventRoomLeave, Dir: DirClientToServer, Ack: true, Payload: "RoomRequest"},
	{Name: EventRoomList, Dir: DirClientToServer},
	{Name: EventRoomJoinSFU, Dir: DirClientToServer, Ack: true},

	{Name: EventRoomCreated, Dir: DirServerToClient},
	{Name: EventRoomLeft, Dir: DirServerToClient},
	{Name: EventRoomUpdated, Dir: DirServerToClient, Payload: "RoomInfo"},
	{Name: EventMemberJoined, Dir: DirServerToClient, Payload: "MemberInfo"},
	{Name: EventMemberLeft, Dir: DirServerToClient},
	{Name: EventMemberUpdated, Dir: DirServerToClient},
	{Name: EventRoomListResult, Dir: DirServerToClient},

	{Name: EventMemberMicState, Dir: DirClientToServer},
	{Name: EventRoomKick, Dir: DirClientToServer},
	{Name: EventRoomKicked, Dir: DirServerToClient},

	{Name: EventMemberSpeaking, Dir: DirClientToServer},
	{Name: EventRoomActiveSpeakers, Dir: DirServerToClient},
	{Name: EventTyping, Dir: DirBoth},

	{Name: EventUserMuted, Dir: DirServerToClient, Payload: "MuteInfo"},
	{Name: EventUserUnmuted, Dir: DirServerToClient, Payload: "MuteInfo"},
	{Name: EventMemberMuted, Dir: DirServerToClient},
	{Name: EventMemberUnmuted, Dir: DirServerToClient},

	{Name: EventSFUProviderChanged, Dir: DirServerToClient},

	{Name: EventMessageSend, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageEdit, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageDelete, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageReact, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageUnreact, Dir: DirClientToServer, Ack: true},
	{Name: EventMessageCreated, Dir: DirServerToClient},
	{Name: EventMessageUpdated, Dir: DirServerToClient},
	{Name: EventMessageDeleted, Dir: DirServerToClient},
	{Name: EventMessageReaction, Dir: DirServerToClient},
	{Name: EventMessageAck, Dir: DirServerToClient},
	{Name: EventMessageError, Dir: DirServerToClient},

	{Name: EventPrivateSend, Dir: DirClientToServer, Ack: true},
	{Name: EventPrivateNew, Dir: DirServerToClient},

	{Name: EventAnnouncementNew, Dir: DirServerToClient},
}
```

`internal/protocol/payloads.go`（结构体从 `signal/types.go` 原样搬入，json tag 一字不改——P1 保持 wire 等价）：

```go
package protocol

type RoomRequest struct {
	Room string `json:"room"`
	// DomainUUID 命名空间隔离：同一房间名在不同 Domain 下视为不同房间。
	DomainUUID string `json:"domain_uuid,omitempty"`
	Password   string `json:"password,omitempty"`
	Identity   string `json:"identity,omitempty"`
	Stream     string `json:"stream,omitempty"`
}

type MemberInfo struct {
	ID          string `json:"id"`
	Identity    string `json:"identity"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Avatar      string `json:"avatar"`
	IsMuted     bool   `json:"isMuted"`
	IsMicMuted  bool   `json:"isMicMuted"`
	JoinedAt    int64  `json:"joinedAt"`
	Stream      string `json:"stream,omitempty"`
}

type RoomInfo struct {
	ID            uint         `json:"id"`
	UUID          string       `json:"uuid"`
	Name          string       `json:"name"`
	DomainUUID    string       `json:"domain_uuid,omitempty"`
	HasPassword   bool         `json:"hasPassword"`
	Description   string       `json:"description"`
	Limit         uint         `json:"limit"`
	Type          string       `json:"type"`
	AudioOnly     bool         `json:"audioOnly"`
	AllowAudience bool         `json:"allowAudience"`
	Members       []MemberInfo `json:"members"`
	Count         int          `json:"count"`
	CreatedAt     int64        `json:"createdAt"`
}

// MuteInfo is the DTO broadcast to clients for mute/unmute events.
type MuteInfo struct {
	UserID    uint   `json:"user_id"`
	Duration  int64  `json:"duration"`
	Permanent bool   `json:"permanent"`
	Reason    string `json:"reason"`
	ExpiresAt string `json:"expires_at,omitempty"`
}
```

- [ ] **步骤 4：signal 包切换为别名**

`internal/signal/events.go` 整文件替换：

```go
package signal

import "GOSpeak/internal/protocol"

// 事件名常量已收编至 internal/protocol（单一事实源）。
// 本文件仅为存量 signal 代码保留别名，勿新增；新代码直接引用 protocol。
const (
	EventConnect    = protocol.EventConnect
	EventDisconnect = protocol.EventDisconnect

	EventRoomCreate  = protocol.EventRoomCreate
	EventRoomJoin    = protocol.EventRoomJoin
	EventRoomLeave   = protocol.EventRoomLeave
	EventRoomList    = protocol.EventRoomList
	EventRoomJoinSFU = protocol.EventRoomJoinSFU

	EventRoomCreated    = protocol.EventRoomCreated
	EventRoomLeft       = protocol.EventRoomLeft
	EventRoomUpdated    = protocol.EventRoomUpdated
	EventMemberJoined   = protocol.EventMemberJoined
	EventMemberLeft     = protocol.EventMemberLeft
	EventMemberUpdated  = protocol.EventMemberUpdated
	EventRoomListResult = protocol.EventRoomListResult

	EventMemberMicState = protocol.EventMemberMicState
	EventRoomKick       = protocol.EventRoomKick
	EventRoomKicked     = protocol.EventRoomKicked

	EventMemberSpeaking     = protocol.EventMemberSpeaking
	EventRoomActiveSpeakers = protocol.EventRoomActiveSpeakers
	EventTyping             = protocol.EventTyping

	EventUserMuted   = protocol.EventUserMuted
	EventUserUnmuted = protocol.EventUserUnmuted
	EventMemberMuted   = protocol.EventMemberMuted
	EventMemberUnmuted = protocol.EventMemberUnmuted

	EventSFUProviderChanged = protocol.EventSFUProviderChanged

	EventMessageSend    = protocol.EventMessageSend
	EventMessageEdit    = protocol.EventMessageEdit
	EventMessageDelete  = protocol.EventMessageDelete
	EventMessageReact   = protocol.EventMessageReact
	EventMessageUnreact = protocol.EventMessageUnreact

	EventMessageCreated  = protocol.EventMessageCreated
	EventMessageUpdated  = protocol.EventMessageUpdated
	EventMessageDeleted  = protocol.EventMessageDeleted
	EventMessageReaction = protocol.EventMessageReaction
	EventMessageAck      = protocol.EventMessageAck
	EventMessageError    = protocol.EventMessageError

	EventPrivateSend = protocol.EventPrivateSend
	EventPrivateNew  = protocol.EventPrivateNew
)
```

`internal/signal/types.go` 整文件替换：

```go
package signal

import "GOSpeak/internal/protocol"

// 协议 DTO 已收编至 internal/protocol。本文件仅为存量 signal 代码保留别名。
type (
	RoomRequest = protocol.RoomRequest
	MemberInfo  = protocol.MemberInfo
	RoomInfo    = protocol.RoomInfo
	MuteInfo    = protocol.MuteInfo
)
```

- [ ] **步骤 5：announcement_service 常量改引 protocol**

`internal/service/announcement_service.go:19` 删除本地 `EventAnnouncementNew = "announcement:new"` 定义，改为引用（保持包内用法不变）：

```go
import "GOSpeak/internal/protocol"

// 原本地常量已收编至 protocol（单一事实源）。
const EventAnnouncementNew = protocol.EventAnnouncementNew
```

若该常量在文件内是 `const` 块成员，保持块结构只替换右值；文件内如有对它的注释一并保留。

- [ ] **步骤 6：运行测试验证通过**

```bash
cd app/server && go test ./internal/protocol/ ./internal/signal/... ./internal/service/... && go build ./... && go vet ./...
```

预期：protocol PASS（新测试），signal/service 存量测试全绿，build/vet 无错误。

- [ ] **步骤 7：Commit**

```bash
git add app/server/internal/protocol app/server/internal/signal/events.go app/server/internal/signal/types.go app/server/internal/service/announcement_service.go
git commit -m "refactor(server): 事件协议收编 internal/protocol 单一事实源"
```

---

### 任务 2：`cmd/gen-protocol` 生成器（TDD）

**文件：**
- 创建：`app/server/cmd/gen-protocol/generate.go`
- 创建：`app/server/cmd/gen-protocol/main.go`
- 创建：`app/server/cmd/gen-protocol/generate_test.go`
- 创建：`app/server/cmd/gen-protocol/testdata/golden/events.ts`（步骤内生成后提交）
- 创建：`app/server/cmd/gen-protocol/testdata/golden/payloads.ts`
- 创建：`app/server/cmd/gen-protocol/testdata/golden/envelope.ts`

- [ ] **步骤 1：编写失败的 golden 测试**

`generate_test.go`：

```go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// goldenTest 用 internal/protocol 真实源码跑生成器，逐字节比对 fixtures。
// fixtures 即将提交的生成物内容——改协议必须显式更新 golden（防手滑改生成物）。
func goldenTest(t *testing.T) {
	pkgDir := filepath.Join("..", "..", "internal", "protocol")
	files, err := Generate(pkgDir)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, name := range []string{"events.ts", "payloads.ts", "envelope.ts"} {
		want, err := os.ReadFile(filepath.Join("testdata", "golden", name))
		if err != nil {
			t.Fatalf("read golden %s: %v", name, err)
		}
		if string(files[name]) != string(want) {
			t.Errorf("%s drifted from golden; run go run ./cmd/gen-protocol -update-golden and commit", name)
		}
	}
}

func TestGolden(t *testing.T)          { goldenTest(t) }
func TestEventsHaveNoInternal(t *testing.T) {
	files, err := Generate(filepath.Join("..", "..", "internal", "protocol"))
	if err != nil {
		t.Fatal(err)
	}
	if contains(files["events.ts"], `"connection"`) || contains(files["events.ts"], `"disconnect"`) {
		t.Error("internal events must not be emitted to TS")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
```

- [ ] **步骤 2：运行确认失败**

```bash
cd app/server && go test ./cmd/gen-protocol/
```

预期：FAIL（Generate 未定义，编译错误）。

- [ ] **步骤 3：实现生成器**

`cmd/gen-protocol/generate.go`（完整实现，约 200 行）：

```go
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
)

type eventEntry struct {
	Name    string
	Dir     string
	Ack     bool
	Payload string
}

type structDef struct {
	Name   string
	Fields []tsField
}

type tsField struct {
	Name     string
	Type     string
	Optional bool
	Doc      string
}

// Generate 解析 pkgDir 下的 Go 源码，返回 文件名→内容。
// 不执行任何代码，仅静态解析：const 事件名、Catalog 复合字面量、struct 定义。
func Generate(pkgDir string) (map[string]string, error) {
	fset := token.NewFileSet()
	entries, err := parser.ParseDir(fset, pkgDir, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", pkgDir, err)
	}
	g := &gen{fset: fset}
	for _, f := range entries {
		for _, file := range f.Files {
			g.walk(file)
		}
	}
	if len(g.catalog) == 0 {
		return nil, fmt.Errorf("no Catalog var found in %s", pkgDir)
	}
	if err := g.validate(); err != nil {
		return nil, err
	}
	return map[string]string{
		"events.ts":   g.emitEvents(),
		"payloads.ts": g.emitPayloads(),
		"envelope.ts": g.emitEnvelope(),
	}, nil
}

type gen struct {
	fset     *token.FileSet
	consts   map[string]string   // const 名 → 字符串值
	structs  map[string]*structDef
	catalog  []eventEntry
	catalogN string              // Catalog 变量所在 struct 字段引用名（未用，留扩展）
}

func (g *gen) walk(file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			g.genDecl(d)
		}
	}
}

func (g *gen) genDecl(d *ast.GenDecl) {
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.ValueSpec:
			for i, name := range s.Names {
				if lit, ok := constString(s.Values[i]); ok && strings.HasPrefix(name.Name, "Event") {
					if g.consts == nil {
						g.consts = map[string]string{}
					}
					g.consts[name.Name] = lit
				}
			}
		case *ast.TypeSpec:
			if st, ok := s.Type.(*ast.StructType); ok {
				def := &structDef{Name: s.Name.Name}
				for _, field := range st.Fields.List {
					f := g.field(field)
					if f != nil {
						def.Fields = append(def.Fields, *f)
					}
				}
				if g.structs == nil {
					g.structs = map[string]*structDef{}
				}
				g.structs[s.Name.Name] = def
			}
		case *ast.ValueSpec:
			// Catalog 变量：var Catalog = []Event{...}
			for i, name := range s.Names {
				if name.Name != "Catalog" {
					continue
				}
				cl, ok := s.Values[i].(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, elt := range cl.Elts {
					e, ok := elt.(*ast.CompositeLit)
					if !ok {
						continue
					}
					entry := eventEntry{}
					for _, kv := range e.Elts {
						p, ok := kv.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						key := p.Key.(*ast.Ident).Name
						switch key {
						case "Name":
							entry.Name = g.strOrConst(p.Value)
						case "Dir":
							entry.Dir = g.strOrConst(p.Value)
						case "Ack":
							if id, ok := p.Value.(*ast.Ident); ok {
								entry.Ack = id.Name == "true"
							}
						case "Payload":
							if id, ok := p.Value.(*ast.Ident); ok && id.Name != `""` {
								entry.Payload = id.Name
							}
						}
					}
					g.catalog = append(g.catalog, entry)
				}
			}
		}
	}
}
```

辅助与 TS 发射（同文件续写）：

```go
func constString(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

// strOrConst 解析字面量或同包常量标识符；其他表达式报错。
func (g *gen) strOrConst(expr ast.Expr) string {
	if v, ok := constString(expr); ok {
		return v
	}
	if id, ok := expr.(*ast.Ident); ok {
		if v, ok := g.consts[id.Name]; ok {
			return v
		}
		// Direction 常量：DirXxx = "xxx"
		if strings.HasPrefix(id.Name, "Dir") {
			return strings.ToLower(strings.TrimPrefix(id.Name, "Dir"))
		}
		return fmt.Sprintf("unresolved identifier %s", id.Name)
	}
	return fmt.Sprintf("unsupported expr %T", expr)
}

func (g *gen) field(f *ast.Field) *tsField {
	if len(f.Names) == 0 {
		return nil // 嵌入字段：P1 的 4 个 DTO 无嵌入；出现即报错
	}
	jsonName, omit, ok := jsonTag(f.Tag)
	if !ok || jsonName == "-" {
		return nil
	}
	typ, err := g.tsType(f.Type)
	if err != nil {
		// 报错信息带位置，零静默兜底
		panic(fmt.Sprintf("%s: field %s: %v", g.fset.Position(f.Pos()), f.Names[0].Name, err))
	}
	return &tsField{
		Name:     jsonName,
		Type:     typ,
		Optional: omit || isPointer(f.Type),
		Doc:      strings.TrimSpace(f.Doc.Text()),
	}
}

func jsonTag(tag ast.Expr) (name string, omit bool, ok bool) {
	lit, ok := tag.(*ast.BasicLit)
	if !ok {
		return "", false, false
	}
	v, _ := strconv.Unquote(lit.Value)
	if !strings.HasPrefix(v, `json:"`) {
		return "", false, false
	}
	parts := strings.Split(strings.TrimPrefix(v, `json:"`), `"`)
	opts := strings.Split(parts[0], ",")
	name = opts[0]
	for _, o := range opts[1:] {
		if o == "omitempty" {
			omit = true
		}
	}
	return name, omit, name != ""
}

func isPointer(t ast.Expr) bool {
	_, ok := t.(*ast.StarExpr)
	return ok
}

var errUnsupported = fmt.Errorf("unsupported type")

// tsType 递归映射 Go 类型 → TS。
func (g *gen) tsType(expr ast.Expr) (string, error) {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string", nil
		case "bool":
			return "boolean", nil
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			return "number", nil
		case "float32", "float64":
			return "number", nil
		case "any", "interface{}":
			return "unknown", nil
		default:
			if _, ok := g.structs[t.Name]; ok {
				return t.Name, nil
			}
			return "", fmt.Errorf("%w: unknown ident %s", errUnsupported, t.Name)
		}
	case *ast.StarExpr:
		inner, err := g.tsType(t.X)
		return inner, err
	case *ast.ArrayType:
		inner, err := g.tsType(t.Elt)
		if err != nil {
			return "", err
		}
		return inner + "[]", nil
	case *ast.MapType:
		key, err := g.tsType(t.Key)
		if err != nil || key != "string" {
			return "", fmt.Errorf("%w: map key must be string", errUnsupported)
		}
		val, err := g.tsType(t.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Record<string, %s>", val), nil
	case *ast.SelectorExpr: // json.RawMessage 等
		if id, ok := t.X.(*ast.Ident); ok && id.Name == "json" && t.Sel.Name == "RawMessage" {
			return "unknown", nil
		}
		return "", fmt.Errorf("%w: selector %s.%s", errUnsupported, t.X, t.Sel)
	case *ast.InterfaceType:
		return "unknown", nil
	default:
		return "", fmt.Errorf("%w: %T", errUnsupported, expr)
	}
}

func (g *gen) validate() error {
	names := map[string]bool{}
	for _, e := range g.catalog {
		if e.Name == "" || names[e.Name] {
			return fmt.Errorf("catalog entry empty/duplicate name: %q", e.Name)
		}
		names[e.Name] = true
		switch e.Dir {
		case "c2s", "s2c", "both", "internal":
		default:
			return fmt.Errorf("event %s: invalid dir %q", e.Name, e.Dir)
		}
		if e.Payload != "" {
			if _, ok := g.structs[e.Payload]; !ok {
				return fmt.Errorf("event %s: payload struct %q not found in package", e.Name, e.Payload)
			}
		}
	}
	return nil
}

func (g *gen) wireEvents() []eventEntry {
	var out []eventEntry
	for _, e := range g.catalog {
		if e.Dir != "internal" {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// tsEventsKey 生成 EVENTS 键名：EventRoomJoinSFU → ROOM_JOIN_SFU。
func tsEventsKey(goConst string) string {
	name := strings.TrimPrefix(goConst, "Event")
	var b strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(name[i-1])
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				b.WriteByte('_')
			} else if prev >= 'A' && prev <= 'Z' && i+1 < len(name) {
				if next := rune(name[i+1]); next >= 'a' && next <= 'z' {
					b.WriteByte('_')
				}
			}
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String())
}

func header(name string) string {
	return "// GENERATED — 手改无效，跑 pnpm gen:protocol\n"
}

func (g *gen) emitEvents() string {
	var b bytes.Buffer
	b.WriteString(header("events"))
	b.WriteString("// WS 事件协议常量：来源 internal/protocol/events.go Catalog\n")
	b.WriteString("export const EVENTS = {\n")
	for _, e := range g.wireEvents() {
		fmt.Fprintf(&b, "\t%s: %q,\n", tsEventsKey(e.Name), e.Name)
	}
	b.WriteString("} as const;\n\n")
	b.WriteString("export type EventName = keyof typeof EVENTS;\n")
	b.WriteString("export type EventDirection = \"c2s\" | \"s2c\" | \"both\";\n\n")
	b.WriteString("export const EVENT_META = {\n")
	for _, e := range g.wireEvents() {
		fmt.Fprintf(&b, "\t%s: { dir: %q, ack: %t },\n", tsEventsKey(e.Name), e.Dir, e.Ack)
	}
	b.WriteString("} as const satisfies Record<EventName, { dir: EventDirection; ack: boolean }>;\n")
	return b.String()
}

func (g *gen) emitPayloads() string {
	var b bytes.Buffer
	b.WriteString(header("payloads"))
	b.WriteString("// WS 事件 payload 类型：来源 internal/protocol/payloads.go\n")
	names := make([]string, 0, len(g.structs))
	for n := range g.structs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		def := g.structs[n]
		if len(def.Fields) == 0 {
			continue
		}
		fmt.Fprintf(&b, "export interface %s {\n", n)
		for _, f := range def.Fields {
			if f.Doc != "" {
				fmt.Fprintf(&b, "\t/** %s */\n", f.Doc)
			}
			opt := ""
			if f.Optional {
				opt = "?"
			}
			fmt.Fprintf(&b, "\t%s%s: %s;\n", f.Name, opt, f.Type)
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}

func (g *gen) emitEnvelope() string {
	var b bytes.Buffer
	b.WriteString(header("envelope"))
	b.WriteString("// ACK 信封约定：handler 返回纯 data、错误走 error 通道；\n// emitAck 成功 resolve 的是 data 本身（非信封）。\n")
	b.WriteString("import type { EventName } from \"./events\";\n\n")
	b.WriteString("export interface AckFrame<T = unknown> {\n\tid: string;\n\tevent: string;\n\tdata?: T;\n\terror?: { code: number; message: string };\n}\n\n")
	b.WriteString("// AckPayload<T>：emitAck 成功路径 resolve 的 data 类型。\n// P1 各事件 data 尚未全部类型化（PayloadOf 于 P2 补齐），统一先 unknown。\n")
	b.WriteString("export type AckPayload<T> = T;\n\n")
	b.WriteString("export type AckResult<E extends EventName = EventName> = AckPayload<unknown>;\n")
	return b.String()
}
```

`cmd/gen-protocol/main.go`：

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var updateGolden bool
	flag.BoolVar(&updateGolden, "update-golden", false, "overwrite testdata/golden instead of app/web/src/protocol/generated")
	flag.Parse()

	pkgDir := filepath.Join("..", "..", "internal", "protocol")
	outDir := filepath.Join("..", "..", "..", "app", "web", "src", "protocol", "generated")
	if updateGolden {
		outDir = "testdata/golden"
	}
	files, err := Generate(pkgDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-protocol: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gen-protocol: %v\n", err)
		os.Exit(1)
	}
	for _, name := range []string{"events.ts", "payloads.ts", "envelope.ts"} {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(files[name]), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "gen-protocol: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("gen-protocol: wrote %d files to %s\n", len(files), outDir)
}
```

注意：`gen.field` 遇到不支持的类型 panic（带 file:line 的 fset.Position）是有意设计——生成期失败即编译期失败，零静默兜底（规格 §6）。

- [ ] **步骤 4：生成 golden fixtures 并确认测试通过**

```bash
cd app/server && go run ./cmd/gen-protocol -update-golden && go test ./cmd/gen-protocol/
```

预期：golden 生成成功；TestGolden PASS、TestEventsHaveNoInternal PASS。人工抽查 golden 内容（事件键名 `ROOM_JOIN_SFU`/`SFU_PROVIDER_CHANGED` 正确、`MuteInfo.user_id` snake_case 保留）。

- [ ] **步骤 5：Commit**

```bash
git add app/server/cmd/gen-protocol
git commit -m "feat(server): gen-protocol 事件协议 TS 生成器（golden 测试）"
```

---

### 任务 3：TS 生成物落地 + 前端切换

**文件：**
- 创建：`app/web/src/protocol/generated/events.ts`（生成器产物）
- 创建：`app/web/src/protocol/generated/payloads.ts`
- 创建：`app/web/src/protocol/generated/envelope.ts`
- 修改：`app/web/src/socket/events.ts`（整文件替换为一行重导出）
- 修改：`app/web/src/socket/wsClient.ts:287`（emitAck 签名收紧）
- 修改：`app/web/src/socket/events.test.ts`（简化为重导出断言）

- [ ] **步骤 1：生成产物**

```bash
cd app/server && go run ./cmd/gen-protocol
```

预期：`app/web/src/protocol/generated/` 出现三份文件。抽查 `EVENTS` 与旧 `socket/events.ts` 的 40 个键值完全一致（旧表缺 `connection`/`disconnect` 属正确——内部事件不产出）。

- [ ] **步骤 2：`socket/events.ts` 切换为重导出**

整文件替换：

```ts
// 事件常量已收编至 protocol/generated（由后端 internal/protocol 生成，CI 拦漂移）。
// 本文件保留原 import 路径，消费点零改动；勿在此新增事件。
export { EVENTS, EVENT_META, type EventName, type EventDirection } from "./generated/events";
```

- [ ] **步骤 3：emitAck 签名收紧（事件名约束）**

`wsClient.ts:287`：

```ts
function emitAck(event: string, payload?: unknown): Promise<any> {
```

改为：

```ts
function emitAck(event: string, payload?: unknown): Promise<any> {
```

说明：`wsClient` 是传输层，不知道业务事件名；事件名约束加在 socketStore 适配层（`socketStore.ts:429` `return adapter.emitAck(event, payload)` 已透传 `EVENTS.*` 字面量类型）。因此本步骤只做 envelope 类型接入——在 `wsClient.ts` 头部加类型导入并让 `WSMessage` 复用生成物：

```ts
import type { AckFrame } from "@/protocol/generated/envelope";
```

`wsClient.ts:4-9` 的 `interface WSMessage` 整体替换为：

```ts
type WSMessage = AckFrame;
```

`emitAck` 返回类型改为 `Promise<any>` → 保持（P2 收紧为 AckPayload），但补注释：

```ts
	// P2 收紧：Promise<AckPayload<PayloadOf[E]>>（payload struct 逐事件补齐后）。
	function emitAck(event: string, payload?: unknown): Promise<any> {
```

- [ ] **步骤 4：运行前端测试**

```bash
pnpm --filter @gospeak/web test
```

预期：全绿。若 `socket/events.test.ts` 因重导出断言失效（原测试直接 import 深层实现），按新形态重写：

```ts
import { describe, expect, it } from "vitest";
import { EVENTS } from "./events";

describe("socket/events re-export", () => {
	it("re-exports generated catalog", () => {
		expect(EVENTS.ROOM_JOIN).toBe("room:join");
		expect(Object.keys(EVENTS).length).toBeGreaterThan(30);
	});
});
```

- [ ] **步骤 5：Commit**

```bash
git add app/web/src/protocol/generated app/web/src/socket/events.ts app/web/src/socket/wsClient.ts app/web/src/socket/events.test.ts
git commit -m "feat(web): EVENTS/payload 切换生成产物，emitAck 接入 AckFrame"
```

---

### 任务 4：ACK 收口（Go handler + 前端消费点，原子切换）

**文件：**
- 修改：`app/server/internal/signal/message_bridge.go`（全文 5 个 handler + 3 个辅助函数）
- 修改：`app/server/internal/signal/private_bridge.go:24-53`
- 修改：`app/server/internal/signal/hub_room_events.go:11-96`（OnRoomLeave 两处 `{"ok",room}`）
- 修改：`app/web/src/stores/chatStore.ts:436-520,891-905`（三处 `resp?.error` → try/catch）
- 修改：`app/web/src/stores/chatStore.domain.test.ts`（适配 reject 语义）

**约定（本任务唯一的 wire 行为变更，双侧同 PR 落地）：**
- 成功：handler `return "", nil` → wire ACK 无 data → `emitAck` resolve `undefined`
- 失败：handler `return "", pkg.NewAppError(code, msg)` → dispatcher 走 `SendErrorACK` → wire ACK 带 `error:{code,message}` → `emitAck` reject `Error(message)`

- [ ] **步骤 1：写失败的 Go 测试**

`app/server/internal/signal/message_bridge_ack_test.go`：

```go
package signal

import (
	"strings"
	"testing"

	"GOSpeak/internal/pkg"
	"GOSpeak/internal/ws"
)

type stubConn struct {
	ws.ClientMessenger
}

func TestOnMessageSendValidationFailsViaErrorPath(t *testing.T) {
	h := &Hub{}
	data, err := h.OnMessageSend(stubConn{}, `{}`)
	if err == nil {
		t.Fatal("expected error path, got nil")
	}
	if data != "" {
		t.Fatalf("expected empty data on error, got %q", data)
	}
	if !strings.Contains(err.Error(), "room is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOnMessageSendNilService(t *testing.T) {
	h := &Hub{}
	if _, err := h.OnMessageSend(stubConn{}, `{}`); err == nil {
		t.Fatal("expected error when msg service unavailable")
	}
}

func TestAckErrorsAreAppError(t *testing.T) {
	h := &Hub{}
	_, err := h.OnMessageEdit(stubConn{}, `{}`)
	var appErr *pkg.AppError
	if !errorsAs(err, &appErr) {
		t.Fatalf("expected *pkg.AppError, got %T: %v", err, err)
	}
}

func errorsAs(err error, target **pkg.AppError) bool {
	e, ok := err.(*pkg.AppError)
	if ok {
		*target = e
	}
	return ok
}
```

- [ ] **步骤 2：运行确认失败**

```bash
cd app/server && go test ./internal/signal/ -run "TestOnMessageSend|TestAckErrors"
```

预期：FAIL——现状 handler 返回 `(data 内嵌 error JSON, nil)`。

- [ ] **步骤 3：改写 message_bridge.go**

统一模式（以 OnMessageSend 为例，其余 4 个 + private 同构替换）：

```go
// OnMessageSend handles message:send events from clients.
// ACK 约定：成功 return "", nil（wire 无 data）；失败 return "", *AppError
// （dispatcher 转 SendErrorACK）。禁止把错误塞进 data。
func (h *Hub) OnMessageSend(c ws.ClientMessenger, data string) (string, error) {
	if h.msgSvc == nil {
		return "", pkg.NewAppError(pkg.INTERNAL_ERROR, "message service unavailable")
	}

	var req messageSendPayload
	if err := parseJSON(data, &req); err != nil || req.Room == "" {
		return "", pkg.NewAppError(pkg.INVALID_PARAMS, "room is required")
	}

	actor, ok := messageActorFromConn(c)
	if !ok {
		return "", pkg.NewAppError(pkg.UNAUTHORIZED, "unauthorized")
	}

	roomUUID, _, roomDomainUUID, err := h.resolveMessageRoom(c, req.Room, req.DomainUUID)
	if err != nil {
		return "", err
	}
	if err := h.checkMessagePerm(c, roomDomainUUID); err != nil {
		return "", err
	}

	if _, err := h.msgSvc.Send(roomUUID, actor, req.Content, req.ReplyTo, req.ClientNonce, req.Mentions); err != nil {
		return "", err
	}
	return "", nil
}
```

注意：`pkg` 包如无 `UNAUTHORIZED` 常量，用 `pkg.FORBIDDEN`（1013）——以 `internal/pkg/errors.go` 实际常量为准，执行时先确认。

辅助函数签名同步变更：

```go
// resolveMessageRoom ... 返回 (roomUUID, roomType, roomDomainUUID, error)。
// 原返回 ackErr string 的路径全部改为 *AppError：
//   未登录        → pkg.NewAppError(pkg.UNAUTHORIZED, "unauthorized")
//   room store 不可用 → pkg.NewAppError(pkg.INTERNAL_ERROR, "room store unavailable")
//   房间不存在     → pkg.NewAppError(pkg.NOT_FOUND, "room not found: <name>")
//   不在房间       → pkg.NewAppError(pkg.INVALID_PARAMS, "not in room: <name>")
func (h *Hub) resolveMessageRoom(c ws.ClientMessenger, roomName, domainUUID string) (roomUUID, roomType, roomDomainUUID string, err error)
```

```go
// checkMessagePerm 域房间/平台房间判定不变，失败统一返回
// pkg.NewAppError(pkg.FORBIDDEN, "permission denied")。
func (h *Hub) checkMessagePerm(c ws.ClientMessenger, roomDomainUUID string) error
```

```go
// messageErrAck 删除。服务层错误直接 return "", err —— dispatcher 的
// pkg.ClientError(err) 已实现同等脱敏（隐藏 5xxx/6xxx 内部细节）。
```

```go
// marshalPolicyAck 删除。policy 判决失败改为：
// return "", pkg.NewAppError(d.Code, d.Message)
```

`messageErrAck(err)` 原调用点改为 `return "", err`。

- [ ] **步骤 4：改写 private_bridge.go OnPrivateMessageSend**

同构替换：nil service / 参数缺失 / 未登录 → `return "", pkg.NewAppError(...)`；`marshalPolicyAck(d), nil` → `return "", pkg.NewAppError(d.Code, d.Message)`；`SendDirect` 失败 → `return "", err`（不再经 ClientError 手工脱敏）；成功 `return marshalAck(dto)` → **保留返回 dto**（成功路径 data 有值是合法的，私聊 ack data=消息 DTO）：

```go
	dto, err := h.convSvc.SendDirect(identity, req.TargetIdentity, req.Content, req.ClientNonce)
	if err != nil {
		return "", err
	}
	return marshalAck(dto)
```

- [ ] **步骤 5：改写 OnRoomLeave 两处 `{"ok",room}`**

`hub_room_events.go:63-68` 与 `:92-95` 统一为返回纯 room 名（data 为 JSON 字符串 `"roomname"`，前端 `tryParse` 得 string）：

```go
	if identity == "" {
		return marshalAck(req.Room)
	}
```

```go
	return marshalAck(req.Room)
```

- [ ] **步骤 6：运行 Go 测试验证**

```bash
cd app/server && go test ./internal/signal/... && go build ./... && go vet ./...
```

预期：新测试 PASS，signal 存量测试全绿。

- [ ] **步骤 7：前端消费点切换 try/catch**

`chatStore.ts` 三处同构替换（以 `:436-455` MESSAGE_SEND 为例）：

```ts
		// 改前
		const resp = await socket
			.emitAck(EVENTS.MESSAGE_SEND, { ... })
			.catch(() => null);
		if (resp?.error) {
			markMessageFailed(client_nonce, serverAckError(resp.error));
			...
		}
```

改为（错误经 reject，`.catch` 已吞的去掉，改 try/catch）：

```ts
		let resp: unknown;
		try {
			resp = await socket.emitAck(EVENTS.MESSAGE_SEND, { ... });
		} catch (err) {
			markMessageFailed(client_nonce, err instanceof Error ? err.message : "发送失败");
			toast.error(err instanceof Error ? err.message : "消息发送失败");
			return;
		}
```

`:510` MESSAGE_SEND 第二处与 `:891` PRIVATE_SEND 同模式。执行时先读这三处完整函数体，保持原失败分支里的其余副作用（`markMessageFailed` 参数、`pendingNonces` 清理）逐行迁入 catch 块，不得丢步骤。

同时确认 `serverAckError` helper 若仅剩这三处调用，删除它及其类型定义。

- [ ] **步骤 8：适配前端单测**

`chatStore.domain.test.ts`（及 grep `resp?.error`/`emitAck` 命中的其他测试）：mock 从 `Promise.resolve({error: ...})` 改为 `Promise.reject(new Error("..."))`，断言失败分支行为不变（`markMessageFailed` 被调、toast 文案）。

- [ ] **步骤 9：全量测试**

```bash
cd app/server && go test ./... && cd ../.. && pnpm --filter @gospeak/web test
```

预期：双侧全绿。

- [ ] **步骤 10：Commit**

```bash
git add app/server/internal/signal app/web/src/stores/chatStore.ts app/web/src/stores/chatStore.domain.test.ts
git commit -m "refactor(protocol)!: ACK 统一纯 data+err 约定，取消 data 内嵌错误旁路"
```

（`!` 标 breaking——ACK data 不再含 `{error,code}` / `{ok,room}`。）

---

### 任务 5：gen-diff 防线

**文件：**
- 修改：根 `package.json` scripts（~26 行处插入）
- 修改：`lefthook.yml` pre-push 块
- 修改：`.github/workflows/ci.yml`（go test 所在 job 增加一步）
- 创建：`app/server/internal/protocol/gen_contract_test.go`

- [ ] **步骤 1：写失败的内存比对测试**

`internal/protocol/gen_contract_test.go`：

```go
package protocol

import (
	"os"
	"path/filepath"
	"testing"

	gen "GOSpeak/cmd/gen-protocol"
)

// TestGeneratedArtifactsFresh 重跑生成器与已提交产物逐字节比对：
// 改了 protocol 包忘记重新生成时，本地测试直接红，不等 CI。
func TestGeneratedArtifactsFresh(t *testing.T) {
	files, err := gen.Generate(".")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	outDir := filepath.Join("..", "..", "..", "app", "web", "src", "protocol", "generated")
	for _, name := range []string{"events.ts", "payloads.ts", "envelope.ts"} {
		want, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("read committed %s: %v（跑 pnpm gen:protocol）", name, err)
		}
		if string(files[name]) != string(want) {
			t.Errorf("%s stale — 跑 pnpm gen:protocol 并提交产物", name)
		}
	}
}
```

- [ ] **步骤 2：运行确认通过**（任务 3 已生成最新产物）

```bash
cd app/server && go test ./internal/protocol/
```

预期：PASS。红则说明产物过期，跑 `pnpm gen:protocol`。

- [ ] **步骤 3：根 package.json 加脚本**

`scripts` 块（`test:server:go` 行后）插入：

```json
		"gen:protocol": "cd app/server && go run ./cmd/gen-protocol",
```

- [ ] **步骤 4：lefthook pre-push 加命令**

`lefthook.yml` pre-push.commands 块（`radius tokens` 之前）插入：

```yaml
    "protocol gen-diff":
      run: pnpm gen:protocol && git diff --exit-code -- app/web/src/protocol/generated
```

- [ ] **步骤 5：ci.yml 加步骤**

在跑 `go test ./...` 的 server job 内（checkout 之后、tests 之前），插入：

```yaml
      - name: Protocol gen-diff
        run: |
          pnpm gen:protocol
          git diff --exit-code -- app/web/src/protocol/generated
```

- [ ] **步骤 6：验证三条防线**

```bash
# 手改一个生成物模拟漂移
echo "// drift" >> app/web/src/protocol/generated/events.ts
cd app/server && go test ./internal/protocol/ ; cd ../..
# 预期：TestGeneratedArtifactsFresh FAIL
pnpm gen:protocol && git diff --exit-code -- app/web/src/protocol/generated
# 预期：diff 有输出 → 非零退出码（防线生效）
git checkout -- app/web/src/protocol/generated
cd app/server && go test ./internal/protocol/ && cd ../..
# 预期：恢复 PASS
```

- [ ] **步骤 7：Commit**

```bash
git add package.json lefthook.yml .github/workflows/ci.yml app/server/internal/protocol/gen_contract_test.go
git commit -m "ci(protocol): gen-diff 三防线（内存比对/pre-push/CI）"
```

---

### 任务 6：收尾验证

**文件：** 无新改动（验证 + 可能的小修）

- [ ] **步骤 1：全量质量门**

```bash
pnpm build && pnpm lint && pnpm typecheck
cd app/server && go test ./... && cd ../..
pnpm test:web
```

- [ ] **步骤 2：集成测试冒烟（消息/房间链路受 ACK 变更影响，必须过）**

```bash
pnpm test:server
```

预期：全绿。若 signal/message 相关集成用例红，按失败 payload 对齐任务 4 的约定修正（不允许改测试断言来迁就实现——先判断是测试覆盖了旧 wire 行为还是实现回归）。

- [ ] **步骤 3：验证成功标准（规格 §8 P1 部分）**

```bash
# 1. 事件名字符串只存在于 protocol 包（signal/notify/service 无裸字符串）
grep -rn '"room:join"\|"message:send"\|"announcement:new"' app/server/internal --include="*.go" | grep -v protocol | grep -v _test
# 预期：零输出
# 2. 生成物是最新的
pnpm gen:protocol && git diff --exit-code -- app/web/src/protocol/generated
```

- [ ] **步骤 4：按需修复 + 最终 commit**

```bash
git add -A && git commit -m "chore(protocol): P1 收尾修正"
```

---

## 自检记录

- **规格覆盖度**：§3.1（任务 1）、§3.2（任务 2/3）、§3.3（任务 4）、§3.4 P1 部分（任务 1 常量收编 + RemoteHook 结构不变仅消裸字符串——announcement 常量已收编，bus 内部事件 struct 化留 P2，规格 §4 P2 表格对应）、§3.5（任务 5）、§5（任务 2 golden + 任务 5 contract + 存量集成）、§6（生成器 panic on unsupported）、§8 成功标准 1/2/5（任务 6 验证）。规格 §3.4 的「gin.go RemoteHook if-else 改常量分派」：`announcement:new` 是其中唯一 wire 事件名字符串，已在任务 1 收编；RemoteHook 本身结构重构属 P2。
- **占位符扫描**：无 TODO/待定；任务 4 步骤 7 的「读三处函数体保持副作用」是明确的执行指令而非占位。
- **类型一致性**：`Generate(pkgDir) (map[string]string, error)` 在任务 2/5 一致；`AckFrame` 名贯穿 wsClient/envelope；signal 别名与 protocol 定义逐字对应。
- **P1/P2 边界**：emitAck 泛型收窄、push 事件 data 内嵌错误（chatStore handleError、socketEvents room.error）、bus 内部事件 struct 化、snake_case 归一 → 全部显式留 P2。
