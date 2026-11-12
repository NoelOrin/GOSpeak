# 邮件配置

邮件服务承担两项功能：注册时的邮箱验证码，以及忘记密码时的验证码下发。

未配置邮件服务时系统同样可以正常使用——注册环节无需验证邮箱，找回密码则只能联系管理员处理。

## 前置条件

准备一个具备发信能力的 SMTP 账号。常见选择如下：

| 服务 | SMTP 地址 | 端口 | 备注 |
|------|-----------|------|------|
| QQ 邮箱 | `smtp.qq.com` | 587 | 密码使用「授权码」，而非登录密码 |
| 163 邮箱 | `smtp.163.com` | 465 或 587 | 同样用授权码 |
| Gmail | `smtp.gmail.com` | 587 | 需要开应用专用密码 |
| 阿里云邮件推送 | `smtpdm.aliyun.com` | 465 | 需要先验证发信域名 |
| 自建 Postfix | 你的地址 | 25 / 587 | 注意反垃圾和 SPF 配置 |

> 国内服务器发往 QQ / 163 邮箱的邮件容易被判定为垃圾邮件。正式部署建议使用专门的邮件推送服务，并正确配置 SPF、DKIM、DMARC。

## 配置

修改 `deploy/.env`：

```ini
EMAIL_ENABLED=true
SMTP_HOST=smtp.qq.com
SMTP_PORT=587
SMTP_USERNAME=your@qq.com
SMTP_PASSWORD=你的授权码
SMTP_FROM=your@qq.com
SMTP_FROM_NAME=GOSpeak
EMAIL_CODE_TTL=10m
EMAIL_SEND_COOLDOWN=60s
EMAIL_CODE_SECRET=<随机字符串>
```

`SMTP_FROM` 通常与 `SMTP_USERNAME` 相同；部分服务商要求发件地址必须是已验证的地址。

`EMAIL_CODE_SECRET` 用于给验证码签名，防止验证码被伪造。取值为一串较长的随机字符串即可，例如：

```bash
openssl rand -hex 32
```

修改完成后重启服务：

```bash
docker compose up -d
```

## 验证配置

通过注册页的「发送验证码」功能验证，检查以下三项：

1. 接口返回 `code: 0`
2. 邮箱已收到邮件（建议先检查垃圾箱）
3. 后端日志没有 SMTP 认证错误

也可以直接调用接口测试：

```bash
curl -X POST https://voice.example.com/api/v1/email/send_code \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com"}'
```

## 后台管理

管理员可以在「管理 → 邮箱」中修改这些配置，修改结果保存到数据库，无需重启。数据库中的配置优先级高于环境变量。

需要恢复环境变量的取值时，在后台将对应配置清空即可。

## 节流参数

| 参数 | 默认 | 作用 |
|------|------|------|
| `EMAIL_CODE_TTL` | `10m` | 验证码有效期 |
| `EMAIL_SEND_COOLDOWN` | `60s` | 同一邮箱的发送间隔 |

接口层另有独立限流：每个 IP 每分钟最多 5 次。该限流写在代码里，不可通过配置修改。

## 常见问题

**报 `535 Authentication failed`**
密码填写有误。QQ / 163 邮箱需要使用「授权码」而不是邮箱登录密码，在邮箱设置中单独开启 SMTP 服务并生成授权码。

**连接超时**
服务器防火墙拦截了出站 587 端口。多数云厂商默认封禁 25 端口，587 通常不受影响，少数厂商例外。换用其他端口测试，或联系服务商确认。

**邮件已发送但进入垃圾箱**
- 发件域名没配 SPF 记录
- 发件内容太像营销邮件
- 用个人邮箱大量发信被限流

使用专门的邮件推送服务可以避免大部分此类问题。

**配置修改后未生效**
Docker 部署需要执行 `docker compose up -d` 重建容器，`restart` 不会重新读取环境变量。如果之前在后台修改过配置，数据库中的值会覆盖环境变量。

## 相关接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/email/send_code` | 发验证码（限流 5 次/分钟） |
| POST | `/api/v1/email/verify_code` | 校验验证码 |
| POST | `/api/v1/email/config` | 读邮件配置（需 `email_config:read`） |
| POST | `/api/v1/email/update-config` | 改邮件配置（需 `email_config:manage`） |

## 下一步

- [OAuth 登录](/guide/oauth) — 第三方登录
- [配置项](/guide/configuration) — 全部环境变量
