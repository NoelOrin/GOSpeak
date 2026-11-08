# 第三方登录

支持 GitHub、Google、QQ 三个平台。配好之后登录页会自动出现对应按钮。

## 它是怎么工作的

1. 用户点「用 GitHub 登录」
2. 跳转到 GitHub 授权页
3. 用户同意后 GitHub 跳回你的服务器
4. 服务器拿 code 换用户信息，找到或创建本地账号，签发令牌

配置全程在后台完成，不用改代码、不用重启。

## 第一步：在第三方平台建应用

三个平台都要填一个「回调地址」，格式固定：

```
https://你的域名/api/v1/oauth/callback/<provider>
```

`<provider>` 只能是 `github`、`google`、`qq` 之一。

举例：

| 平台 | 回调地址 |
|------|---------|
| GitHub | `https://voice.example.com/api/v1/oauth/callback/github` |
| Google | `https://voice.example.com/api/v1/oauth/callback/google` |
| QQ | `https://voice.example.com/api/v1/oauth/callback/qq` |

建完应用你会拿到 `Client ID` 和 `Client Secret`，留着下一步用。

> 本地调试也能用，回调地址填 `http://localhost:8998/api/v1/oauth/callback/github`。但 GitHub 只接受已备案的域名或者 localhost，其他 IP 不行。

## 第二步：在后台填进去

登录管理员账号，进「管理 → OAuth」，选平台、粘贴 Client ID / Secret，保存。

也可以用接口：

```bash
curl -X POST https://voice.example.com/api/v1/oauth/admin/providers \
  -H "Authorization: Bearer <你的令牌>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "client_id": "你的-client-id",
    "client_secret": "你的-client-secret",
    "redirect_url": "https://voice.example.com/api/v1/oauth/callback/github",
    "enabled": true
  }'
```

各平台的授权地址、换 token 地址、取用户信息地址都是内置的，不用填。只有想接别的平台才需要自己写全。

## 第三步：验证

退出登录，回到登录页。应该能看到「使用 GitHub 登录」的按钮。

点进去走一遍授权，能登进来就成了。

## 用户是怎么关联的

第一次用某个平台登录时，会按「平台 + 平台用户 ID」找有没有绑定过：

- 绑定过 → 直接登进对应账号
- 没绑定 → 新建一个本地账号，用户名取平台昵称（重名会自动加后缀）

同一个人用 GitHub 和 Google 分别登录，默认会得到**两个独立账号**。想要合并得手动处理，目前没有自动合并。

## 几个坑

**点了按钮报 400 / 500**
大概率是回调地址和平台上填的不一致。注意协议（http/https）、域名、末尾斜杠都要一样。

**QQ 登录特别容易失败**
QQ 互联要求域名已备案，且回调地址必须和申请时填的完全一致，连端口都不能差。

**Google 报 redirect_uri_mismatch**
Google Cloud Console 里的「已获授权的重定向 URI」要一字不差地加上回调地址。

**配置改了但按钮没出现**
登录页的按钮来自 `/api/v1/oauth/providers`，这个接口有缓存。硬刷新一下页面。

**想让某个平台暂时下线**
把 provider 的 `enabled` 改成 `false` 就行，配置留着，随时能开回来。

## 相关接口

| 方法 | 路径 | 权限 | 用途 |
|------|------|------|------|
| GET | `/api/v1/oauth/providers` | 公开 | 登录页拿有哪些平台可用 |
| GET | `/api/v1/oauth/login/:provider` | 公开 | 发起授权跳转 |
| GET | `/api/v1/oauth/callback/:provider` | 公开 | 平台回调 |
| GET | `/api/v1/oauth/admin/providers` | `oauth:read` | 后台列表 |
| POST | `/api/v1/oauth/admin/providers` | `oauth:manage` | 新增 |
| PUT | `/api/v1/oauth/admin/providers` | `oauth:manage` | 修改 |
| DELETE | `/api/v1/oauth/admin/providers/:id` | `oauth:manage` | 删除 |

## 下一步

- [邮件配置](/guide/email) — 注册验证码和找回密码
- [权限说明](/guide/permissions) — 谁能改这些配置
