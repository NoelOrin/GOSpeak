# 前端 Socket.IO 信令对接设计

> **项目**: GoRTC  
> **日期**: 2026-05-27  
> **版本**: v1.0  
> **状态**: 设计阶段  
> **前置**: 后端 Socket.IO 信令系统已完成（`feature/socket-sign` 分支）

---

## 1. 概述

### 1.1 目标

将后端已实现的 Socket.IO 信令系统对接到前端，实现：
- 房间实时创建/加入/离开
- 成员在线状态实时同步
- 房间列表实时更新
- 为后续 LiveKit Data Channel 音频状态广播做准备

### 1.2 技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| SolidJS | ^1.9.9 | UI 框架 |
| socket.io-client | ^4.x（新增） | Socket.IO 客户端 |
| livekit-client | ^2.16.1 | WebRTC 媒体（已有） |
| TanStack Query | ^5.90.15 | HTTP 数据获取（已有） |

### 1.3 前置条件

- Vite 已配置 `/socket.io` WebSocket 代理 → `http://localhost:8998`
- 后端 Socket.IO Hub 已实现 12 个事件（`room:create/join/leave/list`、`room:created/joined/left/updated`、`member:joined/left`、`room:list:result`）
- 后端 Token 响应已对齐前端格式：`{ token, serverUrl, room, identity }`

---

## 2. 架构设计

### 2.1 数据流

```
┌──────────────────────────────────────────────────────────────┐
│                        前端 (SolidJS)                        │
│                                                              │
│  ┌─────────────┐    ┌──────────────┐    ┌─────────────────┐ │
│  │ roomDetail  │───►│ socketStore  │───►│  Socket.IO Hub  │ │
│  │   .tsx      │    │ (全局单例)    │    │   (后端)        │ │
│  └──────┬──────┘    └──────┬───────┘    └─────────────────┘ │
│         │                  │                                 │
│         │           ┌──────▼───────┐                         │
│         │           │ voiceChat    │                         │
│         │           │   .tsx       │                         │
│         │           └──────────────┘                         │
│         │                                                    │
│  ┌──────▼──────┐    ┌──────────────┐    ┌─────────────────┐ │
│  │ LiveKit     │───►│ createRoom() │───►│  LiveKit SFU    │ │
│  │ Token API   │    │              │    │  (WebRTC)       │ │
│  └─────────────┘    └──────────────┘    └─────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

### 2.2 双通道架构

| 通道 | 协议 | 职责 | 连接时机 |
|------|------|------|---------|
| **Socket.IO** | WebSocket | 房间管理、成员状态、实时通知 | 进入房间页时 |
| **LiveKit** | WebRTC | 音视频媒体传输 | 点击"加入"按钮时 |

两个通道独立连接，Socket.IO 负责"谁在哪个房间"，LiveKit 负责"音频怎么传"。

---

## 3. 新增文件

### 3.1 `src/stores/socketStore.ts` — Socket.IO 连接管理

全局单例 store，职责：
- 管理 Socket.IO 连接生命周期
- 维护响应式状态（连接状态、房间列表、当前房间、成员列表）
- 提供房间操作方法

**状态：**

```ts
connected: Accessor<boolean>      // Socket.IO 连接状态
rooms: Accessor<RoomInfo[]>       // 所有房间列表
currentRoom: Accessor<string|null> // 当前所在房间名
members: Accessor<MemberInfo[]>   // 当前房间成员列表
```

**方法：**

```ts
connect(token?: string)                    // 建立连接
disconnect()                               // 断开连接
createRoom(name: string)                   // 创建房间
joinRoom(room: string, identity: string)   // 加入房间
leaveRoom(room: string)                    // 离开房间
listRooms()                                // 请求房间列表
```

**事件监听：**

| 后端事件 | 处理逻辑 |
|---------|---------|
| `room:created` | 追加到 `rooms` |
| `room:updated` | 更新 `rooms` 中对应房间 |
| `room:joined` | 设置 `currentRoom` + `members` |
| `room:left` | 清空 `currentRoom` + `members` |
| `member:joined` | 追加到 `members` |
| `member:left` | 从 `members` 移除 |
| `room:list:result` | 替换 `rooms` |

---

## 4. 修改文件

### 4.1 `src/types/room.ts` — 新增类型

```ts
// Socket.IO 信令相关类型
export type MemberInfo = {
  id: string;        // Socket.IO 连接 ID
  identity: string;  // 用户标识
  joinedAt: number;  // 加入时间戳 (ms)
};

export type RoomInfo = {
  name: string;
  members: MemberInfo[];
  count: number;
  createdAt: number;
};
```

### 4.2 `src/components/room/roomDetail.tsx` — 核心改造

**当前问题：**
- 调用不存在的 `POST /api/v1/room/create` 端点
- Token 查询依赖 create 查询成功（`enabled: createRoomQuery.isSuccess`）
- 无 Socket.IO 集成

**改造方案：**

```
改造前:
  createRoom(HTTP) → enabled → getToken(HTTP) → createRoom(LiveKit) → joinRoom

改造后:
  socketStore.connect() → getToken(HTTP) → socketStore.joinRoom() → createRoom(LiveKit) → joinRoom
```

**关键变更：**
1. 移除 `createRoomQuery`（不存在的端点）
2. Token 查询独立执行，不依赖其他查询
3. Token 获取成功后同时触发 Socket.IO `joinRoom` 和 LiveKit `createRoom`
4. `onCleanup` 同时断开 Socket.IO 和 LiveKit

### 4.3 `src/components/room/voiceChat.tsx` — 成员列表

**当前问题：**
- 硬编码 `memberList = [1, 2, 5, 12, 4]`
- `MemberCard` 显示占位图片，无真实数据

**改造方案：**
- 从 `socketStore.members()` 获取真实成员列表
- `MemberCard` 接收 `MemberInfo` 属性，显示 `identity`
- 使用默认头像（无头像时）
- 保留"踢出"和"静音"按钮（暂不实现功能）

### 4.4 `src/components/room/roomList.tsx` — 实时房间列表

**当前问题：**
- 通过 `GET /api/v1/room/list` 单次 HTTP 查询
- 该端点查询的是数据库中的房间，不是 Socket.IO Hub 中的实时房间

**改造方案：**
- 使用 `socketStore.rooms()` 替代 HTTP 查询
- 进入页面时调用 `socketStore.listRooms()` 触发初始加载
- `room:created` / `room:updated` 事件自动更新列表
- 显示每个房间的成员数量和成员列表

---

## 5. 事件流时序

### 5.1 用户 A 创建并加入房间

```
用户 A                     前端                         后端
  │                         │                            │
  │  进入房间页              │                            │
  │────────────────────────►│  socket.connect()          │
  │                         ├───────────────────────────►│
  │                         │                            │
  │                         │  POST /signal/token        │
  │                         ├───────────────────────────►│
  │                         │◄─── { token, serverUrl } ──┤
  │                         │                            │
  │  点击"加入"              │                            │
  │────────────────────────►│  socket.joinRoom("r1")     │
  │                         ├───────────────────────────►│
  │                         │◄─── room:joined { members }┤
  │                         │                            │
  │                         │  createRoom({ token, url })│
  │                         │  room.connect()            │
  │                         │                            │
  │  看到成员列表             │                            │
  │◄────────────────────────│                            │
```

### 5.2 用户 B 加入同一房间

```
用户 B                     前端 B                       后端              前端 A
  │                         │                            │                  │
  │  点击"加入"              │                            │                  │
  │────────────────────────►│  socket.joinRoom("r1")     │                  │
  │                         ├───────────────────────────►│                  │
  │                         │◄─── room:joined { members }┤                  │
  │                         │                            │  member:joined   │
  │                         │                            ├─────────────────►│
  │                         │                            │                  │
  │  看到 A 和 B 的成员列表   │                            │  更新 members()  │
  │◄────────────────────────│                            │                  │
```

### 5.3 用户 B 离开房间

```
用户 B                     前端 B                       后端              前端 A
  │                         │                            │                  │
  │  关闭页面 / 点击离开      │                            │                  │
  │────────────────────────►│  socket.leaveRoom("r1")    │                  │
  │                         ├───────────────────────────►│                  │
  │                         │                            │  member:left     │
  │                         │                            ├─────────────────►│
  │                         │                            │                  │
  │                         │                            │  更新 members()  │
  │                         │                            │  移除 B          │
```

---

## 6. 与 LiveKit 的协同

| 场景 | Socket.IO | LiveKit |
|------|-----------|---------|
| 进入房间页 | 连接、加入房间 | 不连接 |
| 点击"加入" | 已在房间 | 连接、启用麦克风 |
| 静音/取消静音 | （后续通过 Data Channel） | TrackMuted/Unmuted |
| 离开房间 | leaveRoom | disconnect |
| 关闭页面 | disconnect | disconnect |

**关键原则**：Socket.IO 先连接，LiveKit 后连接。Socket.IO 管"谁在"，LiveKit 管"声音怎么传"。

---

## 7. 验证计划

| 测试项 | 验证方法 | 预期结果 |
|--------|---------|---------|
| Socket.IO 连接 | 打开浏览器 DevTools Network | WebSocket 连接建立 |
| 创建房间 | 用户 A 创建房间 | 房间列表实时更新 |
| 加入房间 | 用户 B 加入同一房间 | A 和 B 都看到对方在成员列表 |
| 离开房间 | 用户 B 关闭页面 | A 的成员列表移除 B |
| 房间列表 | 打开房间列表侧边栏 | 显示所有 Socket.IO Hub 中的房间 |
| LiveKit 音频 | 加入后说话 | 音频正常传输（不受 Socket.IO 影响） |
| 断线重连 | 断开网络后恢复 | Socket.IO 自动重连，状态恢复 |
| 前端构建 | `pnpm build` | 无编译错误 |

---

## 8. 后续演进

本次实现完成后，可继续：

1. **LiveKit Data Channel 音频状态广播** — 静音/说话状态通过 Data Channel 广播，更新 `voiceChatStore.otherMemberState`
2. **LiveKit Webhook 后端感知** — 后端记录参与者加入/离开日志
3. **房间创建 UI** — 新建房间弹窗，调用 `socketStore.createRoom(name)`
4. **踢出/静音功能** — 通过 SFU Provider 的 `RemoveParticipant` / `MuteParticipant` 实现

---

*文档结束*
