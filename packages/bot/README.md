# @gospeak/bot

GOSpeak 语音房机器人运行时：插件、Socket 文本桥、旁听/Speech/TTS、Capability Router。

## 架构

```text
Go Server (JWT / RBAC / Room / Signal Hub)
        │ JWT + Socket.IO (+ 业务 REST)
        ▼
Bot Runtime Host
  Auth · PluginManager · EventBus
  CapabilityRouter · Scheduler
  MediaListen · Speech · AudioPublish/TTS
        │ ctx.*
        ▼
Plugins (welcome / moderation / listen-manager / voice-react / …)
```

**不变式**

1. 插件只通过 `ctx.*`，不随意 `fetch` 任意路径  
2. 房内互动只走 Socket：`bot:command` / `bot:message` / `room:kick`  
3. `pcmHub.publish` 是进程内总线，不是推到 SFU  
4. 推到 SFU 必须走 PublishAdapter  
5. Bot 先信令在场，再谈收发与媒体  
6. 忽略自身 identity，防自激  

**没有** bot REST 文本桥（无 `POST /api/v1/bot/command|message|kick`）。

## 快速启动

```bash
cd packages/bot
cp .env.example .env
# 填 GOSPEAK_TOKEN 或 GOSPEAK_BOT_USERNAME/PASSWORD
pnpm start
```

## 权限与创建 Bot

Bot token 白名单：`room:read`, `room:create`, `user:read`, `signal:kick`, `mute:manage`。

通过管理端 `POST /api/v1/bot/create` 创建，并授予上述权限。

## 环境变量

| 变量 | 说明 |
|------|------|
| `GOSPEAK_SERVER_URL` | HTTP API |
| `GOSPEAK_SOCKET_URL` | Socket.IO |
| `GOSPEAK_TOKEN` / `GOSPEAK_BOT_USERNAME`+`PASSWORD` | 鉴权 |
| `GOSPEAK_BOT_IDENTITY` | 身份名 |
| `GOSPEAK_PLUGIN_DIR` | 插件目录 |
| `GOSPEAK_AUTO_JOIN_ROOMS` | 启动信令 join，逗号分隔 |
| `GOSPEAK_LISTEN_ROOMS` | 启动旁听房间 |
| `GOSPEAK_ENABLE_LISTEN` | 强制开旁听管线 |
| `GOSPEAK_ENABLE_SPEAK` | TTS + publishPcm |

## 插件编写

```ts
import { Plugin } from "@gospeak/bot/core";
import { On, Command } from "@gospeak/bot/decorators";
import { EventType } from "@gospeak/bot/core";
import { RegisterPlugin } from "@gospeak/bot/decorators";

@RegisterPlugin({ name: "demo", author: "you", desc: "demo", version: "1.0.0" })
export class Demo extends Plugin {
  @On(EventType.OnMemberJoined)
  async hi(e: any) {
    await this.ctx.chat.send(e.room.id, `hi ${e.actor?.name}`);
  }

  @Command("ping")
  async ping(e: any) {
    await this.ctx.chat.reply(e, "pong");
  }
}
```

### 关键 ctx

- `ctx.chat.send/reply` → socket `bot:message`
- `ctx.voice.removeMember` → socket `room:kick`
- `ctx.voice.muteMember` → REST mute
- `ctx.voice.speak/publishPcm` → TTS/PublishAdapter（需 enableSpeak）
- `ctx.rooms.*` → REST + join/leave
- `ctx.users.getByIdentity` → REST
- `ctx.mutes.list/status` → REST
- `ctx.listen.add/remove/list/clear` → 旁听集合
- `ctx.scheduler.every/once` → 定时任务

### 内置插件

| 插件 | 作用 |
|------|------|
| welcome | `OnMemberJoined` 欢迎 |
| moderation | `/kick` `/mute` 等 |
| room-manager | `/room create|list|…` |
| mute-manager | `/gmute` 服务端禁言 |
| keyword-reply | 关键词 |
| listen-manager | `/listen add|remove|list|clear` |
| voice-react | 唤醒词 / 违规词 / 语音踢人 |
| idle-guard | 定时巡检 |

## 测试

```bash
cd packages/bot && pnpm test
cd app/server && go test ./internal/signal -count=1
```
