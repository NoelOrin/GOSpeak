# 错误码

接口返回的 `code` 字段是业务错误码，与 HTTP 状态码相互独立——HTTP 状态码可能为 200，而 `code` 非 0 表示业务上失败。

## 0 — 成功

```json
{ "code": 0, "msg": "success", "data": { ... } }
```

## 1xxx — 认证与账号

| 码 | 常量 | 含义 | 处理方式 |
|----|------|------|---------|
| 1001 | `TOKEN_NOT_EXIST` | 请求未携带令牌 | 登录获取令牌 |
| 1002 | `TOKEN_WRONG` | 令牌签名校验不通过 | 令牌被篡改或签名密钥已更换，需重新登录 |
| 1003 | `TOKEN_EXPIRED` | 令牌已过期 | 使用 refresh_token 换取新令牌 |
| 1010 | `INVALID_PASSWORD` | 密码错误 | 向用户提示 |
| 1011 | `USER_NOT_FOUND` | 用户不存在 | — |
| 1012 | `USERNAME_EXISTS` | 用户名已被占用 | 更换用户名 |
| 1013 | `FORBIDDEN` | 权限不足 | 当前角色没有对应权限 |
| 1014 | `TOKEN_REVOKED` | 令牌已吊销 | 已登出或已修改密码导致，需重新登录 |
| 1015 | `USER_BANNED` | 账号被封禁 | 联系管理员 |
| 1016 | `USER_MUTED` | 已被禁言 | 禁言期间无法发言 |
| 1017 | `RATE_LIMITED` | 请求过于频繁 | 稍后重试 |

**1003 与 1014 的差异**：1003 为自然过期，1014 为主动吊销（登出、修改密码导致 `token_version` 变化）。

## 2xxx — 参数

| 码 | 常量 | 含义 |
|----|------|------|
| 2001 | `INVALID_PARAMS` | 参数校验失败，`msg` 中说明具体原因 |

## 3xxx — 资源

| 码 | 常量 | 含义 |
|----|------|------|
| 3001 | `NOT_FOUND` | 资源不存在 |
| 3002 | `ALREADY_EXISTS` | 资源已存在 |

## 5xxx — 服务端

| 码 | 常量 | 含义 |
|----|------|------|
| 5001 | `INTERNAL_ERROR` | 服务器内部错误，需查看服务端日志 |

## 6xxx — 语音服务

| 码 | 常量 | 含义 | 处理方式 |
|----|------|------|---------|
| 6001 | `SFU_NOT_CONFIGURED` | 未配置 SFU | 检查 `LIVEKIT_HOST` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` |
| 6002 | `SFU_ERROR` | 调用 LiveKit 失败 | 检查 LiveKit 服务状态与密钥配置 |

## 7xxx — 第三方登录

| 码 | 常量 | 含义 |
|----|------|------|
| 7001 | `OAUTH_PROVIDER_NOT_FOUND` | 对应平台未配置 |
| 7002 | `OAUTH_PROVIDER_DISABLED` | 平台已配置但被禁用 |
| 7003 | `OAUTH_TOKEN_EXCHANGE_FAILED` | 换取 token 失败，通常为 secret 不正确或回调地址不匹配 |
| 7004 | `OAUTH_GET_USER_FAILED` | 已获取 token 但无法取得用户信息 |

## 8xxx — 邮件

| 码 | 常量 | 含义 |
|----|------|------|
| 8001 | `EMAIL_ALREADY_EXISTS` | 邮箱已被注册 |
| 8002 | `EMAIL_CODE_INVALID` | 验证码不正确 |
| 8003 | `EMAIL_CODE_EXPIRED` | 验证码已过期 |
| 8004 | `EMAIL_CODE_ALREADY_USED` | 验证码已被使用 |
| 8005 | `EMAIL_SEND_TOO_FREQUENT` | 发送过于频繁 |
| 8006 | `EMAIL_SEND_FAILED` | 发信失败，SMTP 配置存在问题 |
| 8007 | `EMAIL_NOT_VERIFIED` | 邮箱未验证 |
| 8008 | `EMAIL_CODE_MAX_ATTEMPTS` | 验证码试错次数已达上限 |
| 8009 | `EMAIL_NOT_CONFIGURED` | 未配置邮件服务 |
| 8010 | `PASSWORD_RESET_DISABLED` | 找回密码功能已关闭 |

## 81xx — 存储

| 码 | 常量 | 含义 |
|----|------|------|
| 8101 | `STORAGE_NOT_CONFIGURED` | 未配置存储 |
| 8102 | `STORAGE_ERROR` | 存储操作失败 |
| 8103 | `STORAGE_FILE_TOO_LARGE` | 文件超出大小限制 |
| 8104 | `STORAGE_FILE_TYPE_NOT_ALLOWED` | 文件类型不被允许 |

## 前端处理约定

按错误码大类处理即可满足大多数场景：

```ts
switch (Math.floor(code / 1000)) {
  case 1:
    // 认证问题：1003 尝试刷新令牌，其余跳转登录
    break;
  case 2:
    // 参数问题：将 msg 展示给用户
    break;
  case 5:
    // 服务端问题：提示用户稍后重试
    break;
}
```

以下特殊情况需要单独处理：

- `1013` 权限不足 → 提示「无对应权限」，不跳转登录
- `1015` 账号被封 → 跳转到说明页，不持续重试
- `1017` 限流 → 倒计时结束后再开放操作

## 下一步

- [API 参考](/reference/api) — 接口列表
- [权限说明](/guide/permissions) — 1013 的具体含义
