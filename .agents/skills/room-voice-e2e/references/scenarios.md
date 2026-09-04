# Room Voice E2E Scenarios

## 前置条件

1. 后端 `app/server` 已启动（默认 `http://localhost:8998`）
2. 前端 `app/web` 已启动（默认 `http://localhost:3000`，Vite 反代 `/api` 与 `/socket.io`）
3. 当前 SFU provider 可用（`livekit` / `srs` / `agora` / ...）
4. 准备至少 1 个可登录用户；多人流测试需要 2 个用户
5. 账号不要处于默认密码强制改密态

## Suite 矩阵

| Suite | 目标 | 最少用户 | 通过标准 |
|-------|------|----------|----------|
| `join` | 创建并进入房间 | 1 | 离开按钮可见 + media ready |
| `switch` | A→B 切房 | 1 | 最终停留 B，UI/媒体正常 |
| `rapid-switch` | A/B 快速来回 | 1 | 每轮都 join 成功，无 failed/retry |
| `media` | 推流/会话 | 1 | getUserMedia + PC/local track |
| `multi-user` | 本地多人拉流 | 2 | 双方成员数≥2，且各自收到远端 audio |

## 1. join

1. 登录
2. 创建唯一房间名 `e2e-join-*`
3. 双击进入
4. 等待 phase 到 interactive（`离开` 可见）
5. 检查 media probe：
   - `getUserMediaCalls >= 1` 或 local audio track live
   - 至少 1 个 RTCPeerConnection（部分 provider 时序差异可放宽到 joined UI + gum）

失败信号：

- 长期停在 `加载语音引擎...` / `连接媒体...`
- 出现 `加入失败` / `重试`
- console 有 token/SFU/ICE 致命错误

## 2. switch

1. 创建 roomA、roomB
2. 进入 roomA，确认 joined
3. 不点离开，直接双击 roomB
4. 确认标题变为 roomB，`离开` 仍可见
5. media probe 仍 ready（允许短暂 reconnecting）

关键回归点：

- 旧房 teardown 与新房 join 竞态
- `leaveRoom` fire-and-forget 不应清掉新房 `currentRoom`

## 3. rapid-switch

1. 创建 roomA、roomB
2. 以短间隔（默认 120ms）在 A/B 间双击切换 N 轮（默认 3）
3. 每次切换都要等到 joined，再切下一个
4. 结束后最终房间 media ready

失败信号：

- 某次切换超时
- UI 显示旧房名但成员/媒体是新房，或反过来
- 多次后必须手动刷新才能恢复

## 4. media（推拉流单人侧）

1. 进房
2. 断言本地采集与发布路径：
   - getUserMedia 被调用
   - 存在 audio local track 或 sender
   - RTCPeerConnection 进入 connecting/connected（或 provider 等价成功态）
3. 单人套件**不能**完整证明远端拉流；拉流以 `multi-user` 为准

可选增强（手动/computer-use）：

- 观察成员卡 speaking 指示
- 切换麦克风静音后 local sender muted/enabled 变化

## 5. multi-user（本地多人房间音频流）

1. Context A 登录 userA，创建 roomM 并加入
2. Context B 登录 userB，加入同一 roomM
3. 双方 UI 显示 ≥2 人在线
4. 双方 media probe 检测到至少 1 路 remote audio（`srcObject` live audio track）
5. 可选：A 离开后，B 远端音频移除、人数回落

失败信号：

- 成员列表互不可见（信令问题）
- 成员可见但无 remote audio（订阅/拉流问题）
- 只有一方能听到（单向订阅/权限/autoplay）

## 报告要求

遵循仓库 `test-logging` skill：

- 路径：`agent_test_logs/room-voice-e2e-YYYY-MM-DD-HH-MM.md`
- 表头含环境、SFU provider、套件结果
- 失败项附截图路径与 probe snapshot 摘要
- token 截断，不写密码

## 判定优先级

1. UI 状态（能否稳定停留在目标房间）
2. 信令成员一致性（多人）
3. 媒体会话（local publish）
4. 远端拉流（multi-user remote audio）
