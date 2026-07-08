# 内置基础能力插件

每个插件独立注册，按功能分类。所有插件继承 `Plugin` 基类，通过 `@RegisterPlugin` + `@Command` / `@On` 装饰器声明能力。

## 插件分类

| 插件 | 目录 | 功能 |
|------|------|------|
| room-manager | `room-manager/` | 语音频道自动管理 |
| keyword-reply | `keyword-reply/` | 关键词自动回复 |
| moderation | `moderation/` | 房间审核控制 |
| welcome | `welcome/` | 新成员欢迎 |

## room-manager — 语音频道自动管理

| 命令 | 权限 | 说明 |
|------|------|------|
| `/room create <name> [limit]` | member | 创建房间 |
| `/room list` | member | 列出活跃房间 |
| `/room members` | member | 列出当前房间成员 |
| `/room limit <number>` | admin | 设置房间人数上限 |

事件：
- `OnRoomJoined` → 自动发送欢迎通知
- `OnRoomLeft` → 成员离开通知

## keyword-reply — 关键词自动回复

| 命令 | 权限 | 说明 |
|------|------|------|
| `/keyword add <trigger> <response>` | moderator | 添加关键词 |
| `/keyword remove <trigger>` | moderator | 删除关键词 |
| `/keyword list` | moderator | 列出所有关键词 |

事件：
- `OnMessageReceived` → 匹配关键词并自动回复（priority -10，低于命令）

关键词存储在 KV 中，跨重启持久化。

## moderation — 房间审核控制

| 命令 | 权限 | 说明 |
|------|------|------|
| `/kick <identity>` | admin | 踢出成员 |
| `/mute <identity> [duration秒]` | admin | 禁言成员（可定时解除） |
| `/unmute <identity>` | admin | 解除禁言 |
| `/volume <identity> <0-100>` | admin | 设置成员音量 |

事件：
- `OnMemberStateChanged` → 追踪静音状态
- `OnRoomLeft` → 清理离开成员的静音状态

## welcome — 新成员欢迎

| 命令 | 权限 | 说明 |
|------|------|------|
| `/welcome set <message>` | moderator | 设置欢迎语（支持 `{name}` 占位符） |
| `/welcome on` | moderator | 开启欢迎功能 |
| `/welcome off` | moderator | 关闭欢迎功能 |
| `/welcome show` | moderator | 查看当前欢迎语 |

事件：
- `OnRoomJoined` → 发送欢迎消息
