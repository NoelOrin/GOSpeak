# 事件协议单一来源（Event Protocol Single Source of Truth）设计规格

- 日期：2026-11-14
- 状态：已批准（头脑风暴定稿）
- 范围：WS 事件协议（事件名 + ACK 约定 + payload 类型）与 bus 内部事件 payload

## 1. 背景与问题

协议契约当前是**双份手工维护**，无生成、无校验，已发生漂移：

| 问题 | 位置 | 证据 |
|------|------|------|
| 事件名双份常量表 | `internal/signal/events.go`（43 常量）vs `app/web/src/socket/events.ts`（40 键） | 前端 `ANNOUNCEMENT_NEW` 在 events.go 不存在（notify 包裸写 `"announcement:new"`）；Go 侧 notify/bus 包绕过常量直接写字符串 |
| ACK handler 返回值约定三种形状 | `message_bridge.go:44-47`（data 内嵌 `{error,code}`）、`hub_room_events.go:63-67`（`{ok,room}`）、dispatcher 统一 ACK | 全靠前端逐点适配；`ok` 字段冗余（收到 ACK 即成功） |
| payload 无契约 | 事件 payload 以 `interface{}` / 手写 TS 类型散布 | bus 订阅方对 `map[string]interface{}`/`float64` 双重 switch 拆箱（`mute_service.go:74-91`、`domain_service.go:87-92`） |
| wire 字段命名不一致 | `MuteInfo` snake_case（`user_id`/`expires_at`）vs `MemberInfo` camelCase | 同一协议两套命名约定 |

这是「猴子补丁盘点」（2026-11-14，四路代码探查）确认的结构性补丁之一：漂移靠人肉发现，payload 结构无强制对齐。

## 2. 决策记录

| 决策点 | 结论 |
|--------|------|
| 覆盖范围 | 全量：事件名 + ACK 封套约定 + payload 类型，含 bus 内部事件消 `interface{}` 拆箱 |
| 单一来源方向 | Go 为源 → 生成 TS |
| 协议兼容约束 | 允许破坏（server 与 web 同仓同发、自部署双侧同步升级） |
| 迁移节奏 | 两阶段渐进（P1 事件名+ACK 全量；P2 payload 按模块） |
| 运行时校验 | 不做（仅编译期类型；生成 TS interface + Go struct） |
| 方案选型 | 方案 A：协议注册表 + 自写 go/ast 生成器（否决 tygo 直译与 JSON Schema 中立层） |

## 3. 架构

### 3.1 单一事实源：`internal/protocol` 包

新增 `app/server/internal/protocol`，取代 `internal/signal/events.go` 与 `signal/types.go` 中的 DTO：

```
internal/protocol/
├── events.go          # 事件常量 + Catalog 声明
├── payloads.go        # wire payload structs（MemberInfo/RoomInfo/MuteInfo/RoomRequest 迁入）
├── internal_events.go # bus 内部事件条目（Dir: Internal，不产出 TS）
└── envelope.go        # ACK 结果约定类型
```

事件目录声明（gen-protocol 用 go/ast 静态解析，不执行代码）：

```go
const (
    EventRoomJoin = "room:join"
    // ...
)

var Catalog = []Event{
    {Name: EventRoomJoin, Dir: ClientToServer, Ack: true,  Payload: "RoomRequest"},
    {Name: EventRoomLeft, Dir: ServerToClient,            Payload: "RoomLeftPayload"},
}

type Event struct {
    Name    string    // wire 事件名
    Dir     Direction // ClientToServer / ServerToClient / Both / Internal
    Ack     bool      // 上行是否需要 ACK
    Payload string    // payloads.go 中的 struct 名；"" 表示无 payload
}
```

约束：

- 事件名常量、payload struct、Catalog 全部集中在此包；Hub 注册 handler、前端 import、bus 内部事件三处消费同一目录
- `signal` 包保留对 protocol 常量的别名引用过渡，存量代码不必一次性改 import
- `connection`/`disconnect` 为服务端内部事件，同样入 Catalog（`Dir: Internal`），不产出 TS

### 3.2 生成器：`app/server/cmd/gen-protocol`

- ~400 行，仅依赖 go/parser + go/ast，解析 protocol 包源码取 `Catalog` 复合字面量与全部 struct 定义
- 产出三份 TS 到 `app/web/src/protocol/generated/`（该目录已存在，手写协议文件不动）：
  - `events.ts`：`EVENTS` 常量对象（含 dir/ack 元数据类型）+ 事件名 union type
  - `payloads.ts`：每个 wire 事件的 payload interface（Internal 事件跳过）
  - `envelope.ts`：ACK 结果约定类型
- 类型映射：`string→string`、`int/uint/int64→number`、`bool→boolean`、`slice→T[]`、`map[string]X→Record<string,X>`、`*T→T | undefined`（字段标 `?`）、嵌套 struct 递归展开；字段名以 json tag 为准
- 生成物文件头标注 `// GENERATED — 手改无效，跑 pnpm gen:protocol`；前端手写 `socket/events.ts` 变为一行重导出（`export { EVENTS } from "./generated/events"`），消费点零改动
- 解析失败（payload 名未定义、不支持的类型）→ 编译期报错，零静默兜底

### 3.3 ACK 约定统一

- 传输层 `ws.ACK{id, event, data?, error?}` 不变
- 新规则：**Ack handler 返回纯 data，错误一律走 err 返回路径**；dispatcher 已有的 `SendErrorACK(id, event, code, msg)` 是唯一错误通道
- 取消两处旁路：`message_bridge.go` 的 data 内嵌 `{error,code}`、`hub_room_events.go` 的 `{ok,room}`
- `envelope.ts` 产出 `AckPayload<T> = { id: string; event: string; data: T }`；前端 `emitAck` 返回类型从 `Promise<any>` 收紧为按事件名分派的 `Promise<AckPayload<PayloadOf<Event>>>`

### 3.4 bus 内部事件

- `internal_events.go` 与 wire 事件同一 Catalog 结构（`Dir: Internal`），generator 跳过 TS 产出
- `bus.Envelope.Payload interface{}` 分阶段替换为具体 struct；拆箱 switch 全部消掉
- `gin.go:222-237` RemoteHook 的 if-else 裸字符串分派改为按内部事件常量分派（结构不变）
- 原则：新增内部事件必须走 catalog + struct，存量随 P2 按模块迁移

### 3.5 CI 防漂移

- `pnpm gen:protocol`（内部 `go run ./cmd/gen-protocol`）后 `git diff --exit-code` 挂进 CI 与 pre-push
- 另加 Go 测试在内存中跑生成器比对已提交产物（本地无 CI 也能拦）
- 前端生成物只读 import，禁止手改

## 4. 迁移计划（两阶段）

| 阶段 | 内容 | 交付物 |
|------|------|--------|
| P1（单 PR） | protocol 包 + Catalog（事件名全量，payload 仅现有 4 个 DTO）+ 生成器 + TS 生成物切换 + ACK 两处旁路收口 + notify 裸字符串 `announcement:new` 收编 + CI gen-diff | 事件名/ACK 单一来源达成，协议行为等价 |
| P2（每模块一 PR） | payload struct 按模块补齐：room → member → mute → message → private → typing → speaking → sfu-provider-changed → announcement；snake_case 字段归一 camelCase；bus 内部事件同步迁移 | payload 全类型化，`socket/types.ts` 手写类型删除 |

P2 每模块固定动作：protocol 包定义/迁移 payload struct → 重新生成 → 前端消费点切到生成类型 → 删旧定义 → 存量集成测试过。

## 5. 测试策略

- 生成器 golden-file 单测（Go fixture → 期望 TS 逐字节比对），覆盖全部类型映射分支
- Go 侧内存生成比对测试（拦忘跑生成器）
- 存量 `test/` 集成测试作回归网，不新增测试场景
- 前端 `events.test.ts` 简化为仅测重导出可用，断言权交给生成物 + CI diff

## 6. 错误处理

- generator 对未定义 payload 名、不支持的 Go 类型（chan/func/any）报错并指明位置
- 前端运行时防线维持现状（本规格不引入 zod）
- CI gen-diff 是唯一漂移防线，不设运行时告警

## 7. 明确不做（YAGNI）

- 不做运行时 zod 校验
- 不做 JSON Schema 中立产物 / 非 TS 客户端支持
- HTTP REST 响应体不纳入（已有统一响应格式，已收敛）
- 不改传输层 `Message`/`ACK` wire 结构

## 8. 成功标准

1. 事件名字符串在仓库中仅 `internal/protocol` 一处定义；notify/bus/signal 前端全部消费常量或生成物
2. `events.ts` 与 `events.go` 不再人工同步——漂移在 CI 被拦截而非人肉发现
3. 全部 wire 事件 payload 有 Go struct 定义与生成的 TS interface；bus 订阅方无 `interface{}` 拆箱
4. ACK 返回值仅一种约定（纯 data + err 路径），三种形状并存消失
5. 存量集成测试全绿，协议行为除 ACK 约定统一外等价
