# @gospeak/bot

GOSpeak 机器人框架，借鉴 AstrBot 的插件系统与底层暴露能力设计。

## 快速开始

```bash
# 安装依赖
pnpm install

# 开发模式
pnpm dev

# 运行测试
pnpm test

# 构建产物
pnpm build

# 启动 Bot（需要先配置环境变量）
pnpm start
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `GOSPEAK_SERVER_URL` | GOSpeak 后端地址 | `http://localhost:8998` |
| `GOSPEAK_SOCKET_URL` | Socket.IO 地址 | 同 `GOSPEAK_SERVER_URL` |
| `GOSPEAK_TOKEN` | Bot 账号 JWT | 必填 |
| `GOSPEAK_BOT_IDENTITY` | Bot 身份标识 | `gospeak-bot` |
| `GOSPEAK_BOT_NAME` | Bot 显示名 | `GOSpeak Bot` |
| `GOSPEAK_PLUGIN_DIR` | 插件目录 | `./plugins` |

## 架构

```
src/
├── core/           # 核心层
│   ├── types.ts        # EventType 枚举 + 事件类型
│   ├── plugin.ts       # Plugin 基类 + 元数据
│   ├── context.ts      # BotContext（底层暴露能力）
│   ├── registry.ts     # 全局注册中心
│   ├── eventBus.ts     # 事件分发引擎
│   └── loader.ts       # 插件加载器
├── filters/        # 过滤器
│   ├── commandFilter.ts    # 命令匹配
│   ├── regexFilter.ts      # 正则匹配
│   ├── permissionFilter.ts # 权限检查
│   └── messageTypeFilter.ts
├── decorators/     # 装饰器
│   ├── register.ts     # @RegisterPlugin
│   └── handlers.ts     # @On / @Command
├── runtime/        # 运行时层
│   ├── apiClient.ts    # REST API 客户端
│   ├── socketClient.ts # Socket.IO 客户端
│   └── botRunner.ts    # Bot 运行器
├── plugins/example/ # 参考插件
└── main.ts         # CLI 入口
```

## 编写插件

```typescript
import { Plugin } from "@gospeak/bot/core";
import { RegisterPlugin, Command } from "@gospeak/bot/decorators";
import { PermissionFilter } from "@gospeak/bot/filters";
import type { MessageEvent } from "@gospeak/bot/core";

@RegisterPlugin({
  name: "my-plugin",
  author: "you",
  desc: "My first GOSpeak bot plugin",
  version: "1.0.0",
})
export class MyPlugin extends Plugin {
  @Command("hello", { alias: ["hi"], desc: "Say hello" })
  async onHello(event: MessageEvent): Promise<void> {
    await this.ctx.chat.reply(event, "Hello, world!");
  }

  @Command("kick", {
    desc: "Kick a member (admin only)",
    filters: [new PermissionFilter("admin")],
  })
  async onKick(event: MessageEvent): Promise<void> {
    const target = event.rawCommand?.args[0];
    if (target) {
      await this.ctx.voice.removeMember(event.room.id, target);
    }
  }
}
```

## BotContext 底层暴露能力

| 能力 | 接口 | 方法 |
|------|------|------|
| 聊天 | `ctx.chat` | `send(roomId, content)`, `reply(event, content)` |
| 房间 | `ctx.rooms` | `listRooms()`, `getMembers(roomId)`, `createRoom(name)` |
| 语音 | `ctx.voice` | `muteMember()`, `removeMember()`, `setMemberVolume()` |
| 存储 | `ctx.kv` | `get(key)`, `set(key, value)`, `delete(key)` |
| 日志 | `ctx.logger` | `debug()`, `info()`, `warn()`, `error()` |
| 权限 | `ctx.hasPermission()` | 检查成员权限等级 |

## 事件类型

| EventType | 触发时机 |
|-----------|----------|
| `OnBotLoaded` | Bot 启动完成 |
| `AdapterMessage` | 收到消息（命令匹配） |
| `OnMessageReceived` | 收到消息（通用） |
| `OnMessageSent` | 消息发送后 |
| `OnRoomCreated` | 房间创建 |
| `OnRoomJoined` | 加入房间 |
| `OnRoomLeft` | 离开房间 |
| `OnMemberStateChanged` | 成员状态变更（静音/音量） |
| `OnPluginLoaded` | 插件加载完成 |
| `OnPluginUnloaded` | 插件卸载完成 |
| `OnPluginError` | 插件处理异常 |
