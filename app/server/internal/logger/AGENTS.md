# Logger Module

统一日志封装，基于 `sirupsen/logrus`。

## 初始化

```go
// 启动时（config 加载后）
logger.Init(logger.OptionsFrom(cfg.LoggerOptions()))
logger.SetupGin()
```

## 用法

```go
logger.Info("plain")
logger.Infof("count=%d", n)
logger.WithComponent("Signal").Infof("client connected: %s", id)
logger.WithFields(logger.Fields{"room": room, "identity": id}).Warn("kick")
logger.WithError(err).Error("failed")
```

## 环境变量

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | dev=`debug` / prod=`info` | `trace\|debug\|info\|warn\|error` |
| `LOG_FORMAT` | dev=`text` / prod=`json` | `text\|json` |
| `LOG_OUTPUT` | `stdout` | `stdout\|stderr\|file\|both` |
| `LOG_FILE` | `logs/app.log` | `file/both` 时路径 |
| `LOG_CALLER` | `false` | 是否打印调用方 |

## Gin

- 使用 `gin.New()` + `logger.GinRecovery()` + `logger.GinLogger()`
- `SetupGin()` 会接管 `gin.DefaultWriter` / `DefaultErrorWriter`
- 标准库 `log` 经 `RedirectStdLog()` 转发到本模块（存量 `log.Printf` 无需立刻改完）

## 约定

- 业务日志优先 `WithComponent("模块名")`
- 生产默认 JSON，便于采集
- 不要在热路径刷 Debug 以外的超高频日志
