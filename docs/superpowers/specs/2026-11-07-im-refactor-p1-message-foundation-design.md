# 文字 IM 重构 — P1 消息地基 设计规格

日期：2026-11-07
状态：已评审（方案 A 通过，§1-§6 用户逐节确认）
后续阶段：P2 导航与频道（前端为主）、P3 体验增强（Typing / 服务端线程 / 全局搜索）各自独立走规格 → 计划 → 实现周期

## 背景与动机

现有文字 IM 存在结构性割裂，与「参考 Discord」的产品思路不符：

1. **两套割裂 UI**：文字房在域工作区下半屏（TextRoomPanel），私聊在独立 `/chat` 页，输入框零复用
2. **会话模型绑用户名**：`ConversationID = md5(排序用户名)`，改名断链；`conversation_participants` 硬编码双列 2 人会话
3. **未读体系只覆盖私聊**：文字频道零未读/红点/提及通知
4. **消息操作残缺**：私聊无 edit/delete/reaction；前端 UI 未接后端已有能力（编辑按钮、reaction 渲染缺失）
5. **voice 房无文字聊天**、bot 消息旁路不落库（bot 不在本期范围）

本次拆分为三阶段，P1（本规格）先解决后端地基：统一会话抽象 + 统一未读 + 消息操作覆盖 DM。P2 再做 Discord 式导航与统一聊天组件，P3 叠加体验增强。

## 已拍板的关键决策

| 决策点 | 结论 |
|--------|------|
| 产品形态 | 全面 Discord 化导航（P2 落地） |
| 会话类型 | 1:1 direct + 群聊 group DM |
| DM 对象来源 | 同域成员（共同所在语音服务器），无好友体系 |
| 本期可选项 | Typing / 服务端线程 / 全局搜索 → 均归 P3 |
| bot 消息入库 | 不在本期 |
| 存量数据 | **不迁移**：旧 md5 会话与新 UUID 永不匹配 → 自然不可见，不删除；房间消息不受影响 |
| 架构方案 | 方案 A：统一会话抽象（conversation 一等实体 + 参与者集合 + 统一 read_state） |

## §1 数据模型

### 新表 `conversations`

| 字段 | 说明 |
|------|------|
| `ID` | 自增主键 |
| `UUID` | 对外暴露的 `conversation_id`，auto-gen，unique |
| `Type` | `direct` \| `group` |
| `LastMessageUUID` / `LastContent` / `LastSenderUUID` / `LastMessageAt` | 列表投影冗余 |
| `CreatedAt` / `UpdatedAt` | |

### 新表 `conversation_members`

| 字段 | 说明 |
|------|------|
| `ConversationUUID` + `UserUUID` | 联合唯一 |
| `InvitedBy` | 拉人者 UUID（direct 会话为空） |
| `JoinedAt` | |

1:1 与群聊同构：direct 是成员数为 2 的参与者集合，group 只是成员数更多。无「群主」概念，任何群成员可 invite / leave。

### 新表 `read_states`

| 字段 | 说明 |
|------|------|
| `UserUUID` + `ScopeType` + `ScopeID` | 联合唯一；`ScopeType` = `room` \| `conversation`；`ScopeID` = 房间 UUID 或会话 UUID |
| `LastReadMessageUUID` | 已读游标，对齐消息 keyset 分页（before=消息 UUID） |
| `UnreadCount` / `MentionCount` | 计数列，更新策略见 §4 |

文字频道与 DM/群聊的未读同构，全部未读汇总可一次查询。

### Message 表

**不加列**。`ConversationID`（`*string`）语义从 md5 hex 变为新 conversation UUID，复合索引 `idx_msg_conversation` 直接复用；`TargetIdentity` 保留用于 1:1 投影。

### 存量处置

- 删除 `conversation_participants` 的 model 与全部代码路径；表留在旧库不动（不 DROP）
- 旧 private 消息（conversation_id = md5 hex）自然不可见，不删除
- 房间消息（room_uuid 路径）完全不受影响
- 群聊人数上限：常量 **50**（发起者 + 49 个被邀请者；direct 不受此限）

## §2 会话服务与 API

### 新端点（全部 POST、JWT protected）

不进访客白名单 → 访客天然禁 DM（GuestGuard 1013）；invite 时的同域校验是第二道保险。

| 端点 | 行为 |
|------|------|
| `POST /api/v1/conversation/create` | `type=direct`：恰好 1 个对方成员，**幂等**（同成员集合复用已有会话）；`type=group`：自己 + 2~49 人，每个被邀请者须与发起者有**任一共同 Domain** 成员关系 |
| `POST /api/v1/conversation/list` | 我的会话列表 + last_message 投影 + 未读/提及数（来自 read_states 关联） |
| `POST /api/v1/conversation/get` | 会话详情 + 成员列表 |
| `POST /api/v1/conversation/invite` | 仅 group；群成员可邀请与自己同域的用户；direct 调用返回 2001 |
| `POST /api/v1/conversation/leave` | 仅 group；移除自己的成员行，消息保留；direct 调用返回 2001；**最后一名成员离开时删除会话行**（成员行随之消失，历史消息不可达） |
| `POST /api/v1/conversation/messages` | 游标分页（before=消息 UUID，limit 钳制与现有 keyset 实现一致，即 [50,200] 默认 100） |
| `POST /api/v1/read-state/mark` | 两 scope 共用：`{scope_type, scope_id, last_read_message_uuid?}`，upsert 归零计数 + last_read 游标取 max |
| `POST /api/v1/read-state/list` | 我的全部 scope 未读汇总（含 scope 投影）+ 总提及数 |

路由放 `internal/router/routes/conversation/`；read-state 因跨 scope（room + conversation）独立为 `internal/router/routes/readstate/`，`internal/router/router.go` 注册两个组。已读标记统一走 `/read-state/mark`，`/conversation/mark-read` 仅作为旧端点兼容保留（见下）。

### 旧端点兼容策略

P1 期间现有前端必须无感工作。`/api/v1/conversation/{list,messages,mark-read,send}` 四个端点**保留原响应格式**，内部改走新模型：

- `send` 收 target identity → 找/建 direct 会话（成员以 UserUUID 记）→ 写消息（conversation_id = 新 UUID）→ 投影回旧格式（含 conversation_id 映射为新 UUID）；`mark-read` 语义不变（内部改写 read_states）
- 旧前端 IndexedDB 缓存的 md5 会话 ID 与新列表对不上时按既有逻辑重新拉列表，无需特殊处理
- **P2 收尾清单**：前端切换到新端点后删除这四个旧端点

## §3 消息权限管道

统一收敛到 service 层 scope 判定（`ConversationType` 路由），REST 与 WS 走同一条管道：

| 操作 | room scope（文字频道） | conversation scope（DM/群聊） |
|------|----------------------|------------------------------|
| 读/发 | 域成员 + `PermMessageRead`/`PermMessageSend` + GuestCaps 短路（现状不变） | 必须是 `conversation_members` 成员 |
| edit | 仅作者（现状不变） | 仅作者（AuthorUUID 匹配） |
| delete | 作者或 `PermMessageDeleteOthers`（现状不变） | **仅作者**——DM/群聊无管理员概念，platform admin 也不能删别人的 DM 消息 |
| react/unreact | 域成员（现状不变） | 成员即可 |
| guest | 沿用 GuestCaps 域开关 | 直接拒绝（guest 不可能成为成员，双保险） |

WS 侧约束：房间消息仍要求连接槽（已 join 该房，现状不变）；`conversation:send` **不要求槽**——未打开 DM 窗口也能收推送、也能发送（REST/WS 等价）。

## §4 未读体系（read_states 更新策略）

- **写路径**：在 `chat.persist` / `chat.private.persist` 异步 job 内，与消息落库同事务执行
  `UPDATE read_states SET unread_count = unread_count + 1, mention_count = mention_count + ? WHERE scope_type=? AND scope_id=? AND user_uuid != 发送者`
  - conversation scope：对全体成员行更新（≤50 行，SQLite 单机事务内可控）
  - room scope：只对**已存在 read_state 行**的用户累加（活跃用户才有行）
- **行创建（惰性）**：
  - conversation scope：`conversation/create` 与 `invite` 时为每个成员初始化行（unread=0）
  - room scope：用户加入文字频道（`room:join`）时初始化该房间行（unread=0）；`/read-state/list` 不为无行者建行（无行 = 未读 0）
  - **收到 mention 强制创建行**（红点不丢）
- **mention 判定**：复用现有 `MessageMention` 表（发送时已插行），排除自己 mention 自己
- **归零**：mark-read 端点 upsert——`unread_count=0`、`mention_count=0`、`last_read_message_uuid` 取 max(传入值, 现有值)

## §5 WS 事件

新增事件常量（`internal/signal/events.go` 与前端 `src/socket/events.ts` 同步）：

| 方向 | 事件 | 说明 |
|------|------|------|
| C→S | `conversation:send` | direct + group 统一发送；兼容期 `private:send` 内部路由到同一 service 方法 |
| S→C | `conversation:new` | 新消息；对每个在线成员定向投递到 `__user:{identity}` 个人房间（复用现有 personal room 机制，键保持 identity 不变，仅扩展为多目标群发）；payload 含会话投影 + 消息 |
| S→C | `conversation:created` | 被拉入群 / 会话建立；payload = 会话投影 |
| S→C | `conversation:member-joined` / `conversation:member-left` | 群成员变动，定向推给全体成员 |

**兼容双发**：direct 会话的消息同时按旧 payload 发 `private:new`（旧前端 P1 期间正常）；P2 切换后删除。

**清理**：删除 `MESSAGE_ACK` 死常量（后端无发送方、前端无监听方）。

## §6 错误处理与测试

### 错误码（不新增）

| 场景 | 码 |
|------|-----|
| 群满员 / 非 group 调 invite、leave / direct 参数非 1 人 | 2001 INVALID_PARAMS |
| 非同域拉人 / 非成员访问会话 | 1013 FORBIDDEN |
| 会话不存在 | 3001 NOT_FOUND |

全部走 `*AppError` + `pkg.HandleError` 现有模式。

### 测试

- **Go 单测**（app/server 同包 `*_test.go`）：
  - conversation service：create 幂等、群上限 50、同域校验、invite/leave、mark-read
  - 权限管道：direct/group × send/edit/delete/react × 非作者删除拒绝
  - read_states：计数累加、mention 排除自己、mark-read 归零、惰性建行、mention 强制建行
  - 旧端点兼容：响应格式字段断言（投影回归）
- **API 集成测试**（`test/`）：`test/conversation` 扩展新端点全链路 + 群聊流；`test/message` 必须保持通过（兼容性回归）
- **前端验收条件**：P1 合入后现有前端零改动可正常使用（/chat 私聊、文字频道消息均不回归）

## 明确不在 P1 范围

- Discord 式导航重构、统一聊天组件、输入框统一（P2）
- Typing 指示、服务端线程、全局搜索/FTS（P3）
- bot 消息入库与 bot 历史读写
- per-channel mute / 通知偏好设置
- 好友体系、跨域 DM、平台级用户目录
- 存量私聊数据迁移（已拍板放弃）
