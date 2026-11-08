# 邮件配置

邮件用来做两件事：注册时的邮箱验证码、忘记密码时收验证码。

不开也能正常用——只是注册不需要验证邮箱，找回密码只能找管理员。

## 需要准备什么

一个能发信的 SMTP 账号。常见选择：

| 服务 | SMTP 地址 | 端口 | 备注 |
|------|-----------|------|------|
| QQ 邮箱 | `smtp.qq.com` | 587 | 密码填「授权码」不是登录密码 |
| 163 邮箱 | `smtp.163.com` | 465 或 587 | 同样用授权码 |
| Gmail | `smtp.gmail.com` | 587 | 需要开应用专用密码 |
| 阿里云邮件推送 | `smtpdm.aliyun.com` | 465 | 需要先验证发信域名 |
| 自建 Postfix | 你的地址 | 25 / 587 | 注意反垃圾和 SPF 配置 |

> 国内服务器发到 QQ / 163 邮箱容易被判垃圾邮件。认真做的话，建议用专门的邮件推送服务，并把 SPF、DKIM、DMARC 配好。

## 配置

改 `deploy/.env`（或 `app/server/.env.prod`）：

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

`SMTP_FROM` 一般和 `SMTP_USERNAME` 一样，有些服务商要求必须是已验证的发件地址。

`EMAIL_CODE_SECRET` 用来给验证码签名，防止被伪造。随便一串长随机值就行：

```bash
openssl rand -hex 32
```

改完重启：

```bash
docker compose up -d
```

## 验证配好了没

用注册页的「发送验证码」，看：

1. 接口返回 `code: 0`
2. 邮箱确实收到信（先看垃圾箱）
3. 后端日志没有 SMTP 认证错误

也可以用接口直接测：

```bash
curl -X POST https://voice.example.com/api/v1/email/send_code \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com"}'
```

## 后台管理

管理员可以在「管理 → 邮箱」里改这些配置，改完存数据库，不用重启。数据库里的配置优先级高于环境变量。

想恢复环境变量的值，去后台把配置清空。

## 两个节流参数

| 参数 | 默认 | 作用 |
|------|------|------|
| `EMAIL_CODE_TTL` | `10m` | 验证码多久失效 |
| `EMAIL_SEND_COOLDOWN` | `60s` | 同一邮箱多久能再发一次 |

接口本身还有一层限流：每个 IP 每分钟最多 5 次。这个写在代码里，改不了。

## 常见问题

**报 `535 Authentication failed`**
密码填错了。QQ / 163 要用「授权码」而不是邮箱登录密码，去邮箱设置里单独开启 SMTP 服务并生成授权码。

**连接超时**
服务器防火墙挡了出站 587 端口。很多云厂商默认封 25 端口，587 一般没问题，但有的会封。换个端口试试，或者联系服务商。

**信发出去了但进了垃圾箱**
- 发件域名没配 SPF 记录
- 发件内容太像营销邮件
- 用个人邮箱大量发信被限流

用专门的邮件推送服务能省掉大部分麻烦。

**改了配置没生效**
Docker 部署要 `docker compose up -d` 重建容器，`restart` 不会重新读环境变量。如果之前在后台改过配置，数据库里的值会覆盖环境变量。

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
