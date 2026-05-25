# server 模块

GoRTC 服务端入口，基于 Gin + Socket.IO 的 WebRTC 信令服务器。

## 目录结构

```
server/
├── main.go               # 程序入口，Swagger 注解定义
├── agent.md              # 模块说明
├── cmd/
│   └── root.go           # Cobra CLI 命令定义（server / version）
├── server/
│   └── gin.go            # Gin 引擎启动、依赖组装、Socket.IO 集成
├── internal/
│   ├── config/           # 配置管理
│   ├── handler/          # HTTP 请求处理层
│   ├── livekit/          # LiveKit WebRTC 服务封装
│   ├── middleware/       # Gin 中间件（JWT 鉴权）
│   ├── model/            # GORM 数据模型
│   ├── pkg/              # 公共工具包（错误码、JWT、响应封装、OAuth 协议）
│   ├── repository/       # 数据访问层
│   ├── router/           # 路由注册
│   ├── service/          # 业务逻辑层
│   └── signal/           # Socket.IO 信令中心
├── docs/                 # Swagger 文档（swagger.json/yaml + docs.go）
├── packages/web/         # Web 前端（Vite + React + TypeScript）
└── test/                 # 测试用例
```

## 启动方式

```bash
go run main.go server          # 生产模式
go run main.go server -e dev   # 开发模式
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| SERVER_PORT | HTTP 监听端口 | 8998 |
| LIVEKIT_HOST | LiveKit 服务器地址 | — |
| LIVEKIT_KEY | LiveKit API Key | — |
| LIVEKIT_SECRET | LiveKit API Secret | — |
| DB_WAL | SQLite WAL 模式开关 | false |

## 依赖关系

```
main.go → cmd/ → server/gin.go
                   ├── internal/repository/ → SQLite/PostgreSQL/MySQL
                   ├── internal/livekit/    → LiveKit 服务
                   ├── internal/service/    → 业务逻辑层
                   ├── internal/handler/    → HTTP 处理层
                   ├── internal/router/     → 路由注册
                   └── internal/signal/     → Socket.IO 信令
```