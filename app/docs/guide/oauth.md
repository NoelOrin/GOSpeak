# OAuth 登录配置

GOSpeak 支持 GitHub、Google、QQ 第三方 OAuth 登录。通过管理后台 API 动态配置，无需重启服务。

## 配置流程

### 1. 在第三方平台注册应用

在每个平台创建 OAuth App，将回调 URL 指向：

```
http(s)://<your-domain>/api/v1/oauth/callback/<provider>
```

例如：`https://gospeak.example.com/api/v1/oauth/callback/github`

### 2. 通过 API 配置 Provider

需要 admin 权限。使用 `POST /api/v1/oauth/admin/providers`：

```json
{
  "name": "github",
  "client_id": "your-client-id",
  "client_secret": "your-client-secret",
  "redirect_url": "https://gospeak.example.com/api/v1/oauth/callback/github",
  "enabled": true
}
```

### 3. 用户登录

前端显示「使用 GitHub 登录」按钮，点击后跳转到：

```
GET /api/v1/oauth/login/github
```

用户授权后自动回调，完成登录或绑定。

## Provider 管理 API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/oauth/admin/providers` | 列出所有 Provider |
| `POST` | `/api/v1/oauth/admin/providers` | 创建 Provider |
| `PUT` | `/api/v1/oauth/admin/providers` | 更新 Provider |
| `DELETE` | `/api/v1/oauth/admin/providers/:id` | 删除 Provider |

## 数据模型

OAuth 配置持久化到数据库 `oauth_providers` 表，支持运行时动态增删改。

```
OAuthProvider {
    id              uint
    name            string    // github / google / qq
    client_id       string
    client_secret   string
    auth_url        string    // 授权端点
    token_url       string    // token 交换端点
    user_info_url   string    // 用户信息端点
    redirect_url    string    // 回调地址
    scopes          string    // 权限范围
    enabled         bool
}

OAuthAccount {
    id              uint
    user_id         uint      // 绑定的本地用户
    provider        string
    provider_uid    string    // 第三方平台用户 ID
    access_token    string
    refresh_token   string
}
```

## 内置 Provider 预设

系统内建各平台默认端点：

| Provider | Auth URL | Token URL | User Info URL |
|----------|----------|-----------|---------------|
| GitHub | `https://github.com/login/oauth/authorize` | `https://github.com/login/oauth/access_token` | `https://api.github.com/user` |
| Google | `https://accounts.google.com/o/oauth2/auth` | `https://oauth2.googleapis.com/token` | `https://www.googleapis.com/oauth2/v2/userinfo` |
| QQ | `https://graph.qq.com/oauth2.0/authorize` | `https://graph.qq.com/oauth2.0/token` | `https://graph.qq.com/user/get_user_info` |

创建 Provider 时只需提供 `client_id`、`client_secret` 和 `redirect_url`，端点 URL 自动填充。
