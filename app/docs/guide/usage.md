# 基本使用

## 注册与登录

### 用户名密码注册

1. 访问 GOSpeak 前端页面
2. 点击「注册」，填写用户名和密码
3. 注册成功后自动登录

### OAuth 第三方登录

支持 GitHub / Google / QQ 一键登录（需管理员在后台配置）：

```
GET /api/v1/oauth/login/github
GET /api/v1/oauth/login/google
GET /api/v1/oauth/login/qq
```

### 首次修改密码

使用默认初始密码或 OAuth 首次登录后，系统会提示修改密码。通过 `POST /api/v1/auth/first_change_password` 完成。

## 语音房间

### 创建房间

1. 点击右下角的 `+` 按钮（FAB）
2. 输入房间名称
3. 可选：设置房间密码
4. 点击创建

创建者自动成为房间管理员，拥有踢人、全员禁言等权限。

### 加入房间

- 从房间列表点击房间卡片，输入密码（如有）即可加入
- 加入后自动进入语音频道，开始发言检测

### 离开房间

点击房间界面中的「离开」按钮，或直接关闭页面，自动离开房间并断开媒体连接。

## 语音控制

### 基础控制

| 操作 | 方式 |
|------|------|
| 麦克风开关 | 点击麦克风图标按钮切换静音 |
| 输出静音 | 点击扬声器图标按钮一键静音所有声音 |
| 独立音量 | 在成员列表中调节每位成员的单独音量（持久化到 IndexedDB） |
| 发言检测 | 正在发言的成员头像周围显示绿色光环 |

### 管理员语音控制

| 操作 | 说明 |
|------|------|
| 踢出成员 | 将指定成员移出房间并断开其媒体连接 |
| 禁言/解禁 | 禁止指定成员发布音频（禁言后仍可收听） |

> **静音 vs 禁言**：前端「静音」是本地播放静音，不影响服务器和其他成员；「禁言」是服务器级限制，被禁言者不可发布音频轨道。

## 用户管理

### 角色体系

| 角色 | 权限 |
|------|------|
| `user` | 基本用户：创建房间、加入房间、使用语音 |
| `admin` | 管理员：用户管理、OAuth 配置、SFU 配置管理 |

管理员可通过 `POST /api/v1/user/update-role` 调整用户角色。

### 用户列表

`POST /api/v1/user/list` 查看所有用户（需认证）。

## 主题切换

GOSpeak 支持明暗主题：

- **Acid**（浅色）：默认主题
- **Synthwave**（深色）：暗色模式

通过页面顶部的主题切换按钮切换，设置持久化到 localStorage。

## 信号与房间状态

### Socket.IO 事件

前端通过 Socket.IO 与后端实时通信：

| 事件 | 方向 | 说明 |
|------|------|------|
| `room:create` | C→S | 创建房间 |
| `room:join` | C→S | 加入房间 |
| `room:leave` | C→S | 离开房间 |
| `room:list` | C→S | 请求房间列表 |
| `room:created` | S→C | 房间已创建 |
| `room:joined` | S→C | 已加入房间（含成员列表）|
| `room:left` | S→C | 已离开房间 |
| `member:joined` | S→C | 其他成员加入 |
| `member:left` | S→C | 成员离开 |
| `member:updated` | S→C | 成员状态更新 |
| `room:updated` | S→C | 房间信息更新 |
| `room:list:result` | S→C | 房间列表结果 |

### 房间数据流

```
用户进入房间页面
  ├─ socketStore.connect()
  ├─ POST /api/v1/signal/token → { token, serverUrl, room, identity }
  ├─ socketStore.joinRoom(room, identity)
  │    ← room:joined { members: [...] }
  ├─ 创建 LiveKit/SRS Room 实例
  └─ 连接媒体并启用麦克风
```
