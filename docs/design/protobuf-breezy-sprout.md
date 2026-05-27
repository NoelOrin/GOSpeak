# Protobuf 改造方案

## Context

GOSpeak 当前所有通信均为 JSON：REST API 使用 Gin + `pkg.Response` 信封，Socket.IO 使用手动 `JSON.stringify`/`json.Unmarshal` + 无类型的 `map[string]interface{}`。前后端类型（`MemberInfo`、`RoomInfo`、`User`、`LoginRequest` 等）完全手动复制，存在字段名不一致、类型不匹配的风险。Socket.IO 事件载荷是最大的薄弱环节——无类型、手动序列化、易出错。

**目标**：引入 protobuf 作为跨语言类型单一事实来源，通过代码生成消除手动类型复制，优先改造 Socket.IO 信令层。

---

## Phase 0: 基础设施搭建（无运行时变更）

### 0.1 目录结构

在 monorepo 根目录创建 `proto/` 目录：

```
GOSpeak/
  proto/
    buf.yaml
    buf.gen.yaml
    gospeak/
      v1/
        common.proto
        user.proto
        room.proto
        signal.proto
        oauth.proto
```

### 0.2 工具链选型

| 工具 | 用途 | 安装方式 |
|------|------|----------|
| `buf` | proto 编译器替代 protoc | npm `@bufbuild/buf` (devDep) |
| `protoc-gen-go` | Go 代码生成 | `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` |
| `@bufbuild/protoc-gen-es` | TypeScript 代码生成 | npm devDep |
| `@bufbuild/protobuf` | TS 运行时库 | `packages/web` dep |
| `google.golang.org/protobuf` | Go 运行时库 | 已有间接依赖，提升为直接依赖 |

### 0.3 配置文件

**proto/buf.yaml**:
```yaml
version: v2
modules:
  - path: gospeak/v1
lint:
  use: STANDARD
breaking:
  use: FILE
```

**proto/buf.gen.yaml**:
```yaml
version: v2
plugins:
  - local: protoc-gen-go
    out: ../packages/server/proto
    opt: paths=source_relative
  - local: protoc-gen-es
    out: ../packages/web/src/proto
    opt: target=ts
```

### 0.4 构建集成

- **根 `package.json`**：添加 `proto:generate`、`proto:lint` 脚本
- **`packages/server/Makefile`**：添加 `proto` target
- **`packages/server/go.mod`**：`google.golang.org/protobuf` 提升为直接依赖
- **`packages/web/package.json`**：添加 `@bufbuild/protobuf` 依赖
- **`.gitignore`**：提交生成代码（便于 CI），不忽略

---

## Phase 1: Socket.IO 信令类型改造（最高优先级）

### 1.1 Wire Format 兼容性关键发现

当前 Go 结构体 JSON tag 使用 **camelCase**：
- `signal/types.go:10` — `json:"joinedAt"` （不是 `joined_at`）
- `signal/types.go:17` — `json:"createdAt"` （不是 `created_at`）

Proto3 默认生成 **snake_case** JSON key。必须使用 `json_name` option 保持兼容：

```protobuf
message MemberInfo {
  string id        = 1;
  string identity  = 2;
  int64  joined_at = 3 [json_name = "joinedAt"];
}

message RoomInfo {
  string          name       = 1;
  repeated MemberInfo members = 2;
  int32           count      = 3;
  int64           created_at = 4 [json_name = "createdAt"];
}
```

### 1.2 Proto 定义 — signal.proto

```protobuf
syntax = "proto3";
package gospeak.v1;
option go_package = "go_rtc/proto/gospeak/v1;gospeakv1";

// Client -> Server
message RoomRequest {
  string room     = 1;
  string identity = 2;
}

message MemberInfo {
  string id        = 1;
  string identity  = 2;
  int64  joined_at = 3 [json_name = "joinedAt"];
}

message RoomInfo {
  string          name       = 1;
  repeated MemberInfo members = 2;
  int32           count      = 3;
  int64           created_at = 4 [json_name = "createdAt"];
}

// Event payloads
message RoomCreatedEvent {
  RoomInfo room  = 1;
  string   error = 2;
}

message RoomJoinedEvent {
  string          room     = 1;
  repeated MemberInfo members = 2;
  int32           count    = 3;
  string          error    = 4;
}

message RoomLeftEvent {
  string room = 1;
}

message MemberJoinedEvent {
  string room     = 1;
  string identity = 2;
  string id       = 3;
}

message MemberLeftEvent {
  string room     = 1;
  string identity = 2;
  string id       = 3;
}

message RoomUpdatedEvent {
  RoomInfo room = 1;
}

message RoomListResultEvent {
  repeated RoomInfo rooms = 1;
  int32           count   = 2;
}
```

### 1.3 服务端改造

**修改文件**：
- `packages/server/internal/signal/types.go` — 删除 `RoomRequest`、`MemberInfo`、`RoomInfo`，改为 type alias 指向生成的 `gospeakv1` 类型
- `packages/server/internal/signal/hub.go` — 所有 `map[string]interface{}` 替换为生成的 proto message struct

**关键变更示例**（hub.go 第 60-64 行）：

```go
// Before:
h.broadcastToRoomLocked(roomName, EventMemberLeft, map[string]interface{}{
    "room":     roomName,
    "identity": member.Identity,
    "id":       s.ID(),
})

// After:
h.broadcastToRoomLocked(roomName, EventMemberLeft, &gospeakv1.MemberLeftEvent{
    Room:     roomName,
    Identity: member.Identity,
    Id:       s.ID(),
})
```

**序列化策略**：Phase 1 继续使用 JSON over Socket.IO。Proto 生成的 Go struct 有 `json:` tag，`json.Marshal` 输出与当前 wire format 一致。无需改传输层。

### 1.4 前端改造

**修改文件**：
- `packages/web/src/stores/socketStore.ts` — 删除 `MemberInfo`、`RoomInfo` interface，导入生成的 protobuf-es 类型
- `packages/web/src/types/room.ts` — 删除重复的 `MemberInfo`、`RoomInfo` 定义

**关键变更示例**（socketStore.ts）：

```typescript
// Before:
interface MemberInfo { id: string; identity: string; joinedAt: number; }
interface RoomInfo { name: string; members: MemberInfo[]; count: number; createdAt: number; }
socket.emit(EVENTS.ROOM_CREATE, JSON.stringify({ room: name }));

// After:
import { MemberInfo, RoomInfo, RoomRequest, RoomCreatedEvent } from "@/proto/gospeak/v1/signal_pb";
socket.emit(EVENTS.ROOM_CREATE, new RoomRequest({ room: name }).toJsonString());
// In event handlers:
socket.on(EVENTS.ROOM_CREATED, (data) => {
  const event = RoomCreatedEvent.fromJson(data);
  // event.room is typed as RoomInfo
});
```

---

## Phase 2: REST API 类型改造（可选，较低优先级）

保持 REST JSON wire format 不变，仅用 proto 生成类型替换内部 DTO。

### 2.1 Proto 定义

- `common.proto` — `ErrorCode` enum、`ApiResponse` 信封
- `user.proto` — `User`、`LoginRequest`、`RegisterRequest`、`AuthResponse`
- `room.proto` — `Room`、`JoinRoomRequest`、`JoinTokenResponse`
- `oauth.proto` — `OAuthProvider`、`OAuthAccount`、`OAuthProviderConfig`、`OAuthUserInfo`

### 2.2 服务端改造

**修改文件**：
- `packages/server/internal/service/auth_service.go` — `LoginRequest`、`RegisterRequest`、`AuthResponse` 替换为生成类型
- `packages/server/internal/handler/auth_handler.go` — 使用生成类型做 `ShouldBindJSON`
- `packages/server/internal/handler/signal_handler.go` — `JoinRoomRequest` 替换
- `packages/server/internal/pkg/errors.go` — `ErrCode` 可选替换为 proto enum

### 2.3 前端改造

**修改文件**：
- `packages/web/src/api/auth.ts` — `LoginReq`、`BackendUser`、`LoginData` 替换为生成类型
- `packages/web/src/api/apiClient.ts` — `Result<T>` 保持不变（信封不值得 proto 化）

---

## Phase 3: 二进制 Socket.IO 传输（可选，性能优化）

仅在 Phase 1-2 稳定后，且 profiling 表明 JSON 序列化是瓶颈时考虑。

- 验证 `go-socket.io` v1.7.0 是否支持 `[]byte` emit
- 前端 `socket.emit(event, uint8Array)` 发送二进制帧
- 如果库不支持二进制，考虑换用 `github.com/zishang520/socket.io` 或 base64 包装

**实际建议**：房间管理信令消息小、频率低，JSON 完全够用。真正的媒体流量走 LiveKit，不走 Socket.IO。不建议追求二进制。

---

## Proto 完整定义

### common.proto

```protobuf
syntax = "proto3";
package gospeak.v1;
option go_package = "go_rtc/proto/gospeak/v1;gospeakv1";

enum ErrorCode {
  ERROR_CODE_UNSPECIFIED       = 0;
  SUCCESS                      = 0;
  TOKEN_NOT_EXIST              = 1001;
  TOKEN_WRONG                  = 1002;
  TOKEN_EXPIRED                = 1003;
  INVALID_PASSWORD             = 1010;
  USER_NOT_FOUND               = 1011;
  USERNAME_EXISTS              = 1012;
  FORBIDDEN                    = 1013;
  TOKEN_REVOKED                = 1014;
  INVALID_PARAMS               = 2001;
  NOT_FOUND                    = 3001;
  ALREADY_EXISTS               = 3002;
  INTERNAL_ERROR               = 5001;
  SFU_NOT_CONFIGURED           = 6001;
  SFU_ERROR                    = 6002;
  OAUTH_PROVIDER_NOT_FOUND     = 7001;
  OAUTH_PROVIDER_DISABLED      = 7002;
  OAUTH_TOKEN_EXCHANGE_FAILED  = 7003;
  OAUTH_GET_USER_FAILED        = 7004;
}
// Note: SUCCESS=0 and ERROR_CODE_UNSPECIFIED=0 need allow_alias = true
```

### user.proto

```protobuf
syntax = "proto3";
package gospeak.v1;
option go_package = "go_rtc/proto/gospeak/v1;gospeakv1";

message User {
  uint32 id         = 1;
  string uuid       = 2;
  string name       = 3;
  string role       = 4;
  int64  created_at = 5;
  int64  updated_at = 6;
}

message LoginRequest {
  string username = 1;
  string password = 2;
}

message RegisterRequest {
  string username = 1;
  string password = 2;
}

message AuthResponse {
  string access_token        = 1;
  string refresh_token       = 2;
  User   user                = 3;
  bool   need_change_password = 4;
}
```

### room.proto

```protobuf
syntax = "proto3";
package gospeak.v1;
option go_package = "go_rtc/proto/gospeak/v1;gospeakv1";

message Room {
  uint32 id         = 1;
  string uuid       = 2;
  string name       = 3;
  uint32 limit      = 4;
  int64  created_at = 5;
  int64  updated_at = 6;
}

message JoinRoomRequest {
  string room     = 1;
  string identity = 2;
}

message JoinTokenResponse {
  string token      = 1;
  string server_url = 2;
  string room       = 3;
  string identity   = 4;
}
```

### oauth.proto

```protobuf
syntax = "proto3";
package gospeak.v1;
option go_package = "go_rtc/proto/gospeak/v1;gospeakv1";

message OAuthProvider {
  uint32 id           = 1;
  string name         = 2;
  string client_id    = 3;
  string auth_url     = 4;
  string token_url    = 5;
  string userinfo_url = 6;
  string redirect_url = 7;
  string scopes       = 8;
  bool   enabled      = 9;
  int64  created_at   = 10;
  int64  updated_at   = 11;
}

message OAuthAccount {
  uint32 id           = 1;
  uint32 user_id      = 2;
  string provider     = 3;
  string provider_uid = 4;
  int64  created_at   = 5;
  int64  updated_at   = 6;
}

message OAuthUserInfo {
  string provider     = 1;
  string provider_uid = 2;
  string username     = 3;
  string avatar       = 4;
  string email        = 5;
}
```

---

## 关键文件清单

| 文件 | 改动 | Phase |
|------|------|-------|
| `proto/buf.yaml` | 新建 | 0 |
| `proto/buf.gen.yaml` | 新建 | 0 |
| `proto/gospeak/v1/*.proto` | 新建（5个文件） | 0 |
| `packages/server/go.mod` | protobuf 提升为直接依赖 | 0 |
| `packages/server/Makefile` | 添加 proto target | 0 |
| `package.json` (root) | 添加 proto:generate 脚本 + devDeps | 0 |
| `packages/web/package.json` | 添加 @bufbuild/protobuf | 0 |
| `packages/server/internal/signal/types.go` | 删除手写类型，改为 alias | 1 |
| `packages/server/internal/signal/hub.go` | map→proto message, parseJSON→json.Unmarshal | 1 |
| `packages/web/src/stores/socketStore.ts` | 删除 interface，用生成类型 | 1 |
| `packages/web/src/types/room.ts` | 删除 MemberInfo/RoomInfo 定义 | 1 |
| `packages/server/internal/service/auth_service.go` | LoginRequest/AuthResponse 替换 | 2 |
| `packages/server/internal/handler/auth_handler.go` | 使用生成类型 | 2 |
| `packages/web/src/api/auth.ts` | LoginReq/BackendUser/LoginData 替换 | 2 |

---

## 风险与缓解

| 风险 | 缓解措施 |
|------|----------|
| Proto JSON key 与现有 wire format 不一致 | `json_name = "joinedAt"` 覆盖默认 snake_case；Step 5 验证 `json.Marshal` 输出 |
| `go-socket.io` 要求 `string` 参数 | Phase 1 继续 JSON 序列化，proto struct 有 `json:` tag 兼容 |
| Go `uint` vs proto `uint32` | 序列化边界做 `uint32(id)` 转换 |
| `time.Time` vs `int64` 毫秒 | 已有信号类型已用 `int64` 毫秒，model 层边界用 `t.UnixMilli()` |
| Password 泄露到 proto | 所有 proto 定义显式排除 password 字段 |

---

## 验证方式

1. **Phase 0**：`cd proto && buf lint` 通过；`buf generate` 后 `go build ./...` 和 `pnpm --filter @go-rtc/web build` 编译通过
2. **Phase 1**：启动 server + web，测试房间创建/加入/离开/列表，确认 Socket.IO 事件正常收发、JSON wire format 无变化
3. **Phase 2**：测试登录/注册/修改密码/刷新 token，确认 REST API 响应格式不变
4. **回归**：现有 Node.js 集成测试 (`packages/server/test/`) 全部通过
