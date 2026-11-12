# 第三方登录

支持 GitHub、Google、QQ 三个平台。配置完成后，登录页会自动出现对应的登录按钮。

## 工作流程

1. 用户点击「用 GitHub 登录」
2. 跳转到 GitHub 授权页
3. 用户同意后 GitHub 跳回服务器回调地址
4. 服务器使用 code 换取用户信息，查找或创建本地账号并签发令牌

全部配置在后台完成，无需修改代码，也无需重启服务。

## 第一步：在第三方平台创建应用

三个平台都需要填写「回调地址」，格式固定：

```
https://你的域名/api/v1/oauth/callback/<provider>
```

`<provider>` 只能是 `github`、`google`、`qq` 之一。

示例如下：

| 平台 | 回调地址 |
|------|---------|
| GitHub | `https://voice.example.com/api/v1/oauth/callback/github` |
| Google | `https://voice.example.com/api/v1/oauth/callback/google` |
| QQ | `https://voice.example.com/api/v1/oauth/callback/qq` |

应用创建完成后会获得 `Client ID` 和 `Client Secret`，供下一步配置使用。

> 本地调试同样支持 OAuth，回调地址填写 `http://localhost:8998/api/v1/oauth/callback/github`。GitHub 只接受已备案的域名或者 localhost，其他 IP 地址无法使用。

## 第二步：在后台填写配置

使用管理员账号登录，进入「管理 → OAuth」，选择平台、粘贴 Client ID / Secret，保存。

也可以通过接口配置：

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

各平台的授权地址、换 token 地址、用户信息地址均为内置值，无需填写。仅当接入其他平台时才需要自行提供完整配置。

## 第三步：验证

退出登录并回到登录页，页面上应出现「使用 GitHub 登录」的按钮。

点击按钮完成一次完整的授权流程，登录成功即表示配置生效。

## 用户关联机制

首次使用某个平台登录时，系统按「平台 + 平台用户 ID」查找已有绑定关系：

- 已绑定 → 直接登录对应账号
- 未绑定 → 新建一个本地账号，用户名取平台昵称（重名自动追加后缀）

同一用户通过 GitHub 与 Google 两个平台登录时，默认会产生**两个独立账号**。账号合并需要手动处理，当前版本没有自动合并机制。

## 常见问题排查

**点击按钮报 400 / 500**
多数情况下是回调地址与平台上填写的地址不一致。协议（http/https）、域名、末尾斜杠都必须完全相同。

**QQ 登录失败率高**
QQ 互联要求域名已备案，且回调地址必须与申请时填写的完全一致，端口也不能有差异。

**Google 报 redirect_uri_mismatch**
Google Cloud Console 中的「已获授权的重定向 URI」需要一字不差地添加回调地址。

**配置已修改但按钮未出现**
登录页的按钮数据来自 `/api/v1/oauth/providers`，该接口存在缓存。硬刷新页面即可看到最新配置。

**需要临时停用某个平台**
将该 provider 的 `enabled` 改为 `false`，配置会保留，之后随时可以重新启用。

## 相关接口

| 方法 | 路径 | 权限 | 用途 |
|------|------|------|------|
| GET | `/api/v1/oauth/providers` | 公开 | 登录页获取可用平台列表 |
| GET | `/api/v1/oauth/login/:provider` | 公开 | 发起授权跳转 |
| GET | `/api/v1/oauth/callback/:provider` | 公开 | 平台回调 |
| GET | `/api/v1/oauth/admin/providers` | `oauth:read` | 后台列表 |
| POST | `/api/v1/oauth/admin/providers` | `oauth:manage` | 新增 |
| PUT | `/api/v1/oauth/admin/providers` | `oauth:manage` | 修改 |
| DELETE | `/api/v1/oauth/admin/providers/:id` | `oauth:manage` | 删除 |

## 下一步

- [邮件配置](/guide/email) — 注册验证码和找回密码
- [权限说明](/guide/permissions) — 谁能改这些配置
