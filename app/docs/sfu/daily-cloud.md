# Daily 云服务

[Daily](https://www.daily.co/) 是一个专注于 WebRTC 的实时音视频 API 平台，提供托管 SFU + REST API。

## 创建 Daily 项目

1. 注册 [Daily Dashboard](https://dashboard.daily.co/)
2. 创建房间，获取 **API Key** 和 **Domain**

## 环境变量配置

```env
SFU_PROVIDER=daily
DAILY_API_KEY=your-daily-api-key
DAILY_DOMAIN=your-domain.daily.co
```

## 特点

| 特性 | 说明 |
|------|------|
| 部署 | 完全托管 |
| API | RESTful 风格，易于集成 |
| 功能覆盖 | Token + 房间/参与者查询可用 |
| 踢人/禁言 | 需通过 Daily REST API 实现（当前未完整实现）|

## 适用场景

- 海外部署（Daily 的服务器主要在欧美）
- 快速原型开发
- 不想管理任何基础设施
