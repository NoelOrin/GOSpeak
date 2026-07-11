# Agora 云服务

[Agora.io](https://www.agora.io/) 是全球领先的实时音视频 PaaS 平台，使用 SDK 集成，完全托管。

## 创建 Agora 项目

1. 注册 [Agora Console](https://console.agora.io/)
2. 创建项目，获取 **App ID** 和 **App Certificate**
3. （可选）开启 Token 认证

## 环境变量配置

```env
SFU_PROVIDER=agora
AGORA_APP_ID=your-app-id
AGORA_APP_CERTIFICATE=your-app-certificate
AGORA_HOST=             # 可选，自定义主机
AGORA_CUSTOMER_ID=      # 可选
AGORA_CUSTOMER_SECRET=  # 可选
```

## 特点

| 特性 | 说明 |
|------|------|
| 部署 | 零运维。Agora 管理全球节点 |
| 计费 | 按分钟计费（音频/视频不同价格）|
| 功能覆盖 | Token 生成 + 基础房间 API 可用 |
| 延迟 | 全球节点，<200ms |
| 穿透 | 内置，不依赖 TURN |

## 限制

- GOSpeak 对 Agora 的支持处于 Medium 成熟度
- 踢人、禁言等功能需要额外开发
- 国内 app 需额外合规审核

## 适用场景

- 不想搭建和管理 SFU 服务器
- 用户分布全球
- 快速验证产品原型
