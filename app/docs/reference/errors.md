# 错误码

接口返回的 `code` 字段是业务错误码，和 HTTP 状态码不是一回事——HTTP 可能是 200，但 `code` 非 0 表示业务上失败了。

## 0 — 成功

```json
{ "code": 0, "msg": "success", "data": { ... } }
```

## 1xxx — 认证与账号

| 码 | 常量 | 意思 | 怎么处理 |
|----|------|------|---------|
| 1001 | `TOKEN_NOT_EXIST` | 没带令牌 | 去登录 |
| 1002 | `TOKEN_WRONG` | 令牌签名不对 | 令牌被篡改或密钥换了，重新登录 |
| 1003 | `TOKEN_EXPIRED` | 令牌过期 | 用 refresh_token 换新的 |
| 1010 | `INVALID_PASSWORD` | 密码错误 | 提示用户 |
| 1011 | `USER_NOT_FOUND` | 用户不存在 | — |
| 1012 | `USERNAME_EXISTS` | 用户名已被占用 | 换个用户名 |
| 1013 | `FORBIDDEN` | 权限不足 | 该角色没有这个权限 |
| 1014 | `TOKEN_REVOKED` | 令牌已吊销 | 登出过或改过密码，重新登录 |
| 1015 | `USER_BANNED` | 账号被封 | 联系管理员 |
| 1016 | `USER_MUTED` | 被禁言 | 禁言期间不能发言 |
| 1017 | `RATE_LIMITED` | 请求太频繁 | 等一会儿再试 |

**1003 和 1014 的区别**：1003 是自然过期，1014 是主动吊销（登出、改密码导致 `token_version` 变化）。

## 2xxx — 参数

| 码 | 常量 | 意思 |
|----|------|------|
| 2001 | `INVALID_PARAMS` | 参数校验失败，`msg` 里会说哪里不对 |

## 3xxx — 资源

| 码 | 常量 | 意思 |
|----|------|------|
| 3001 | `NOT_FOUND` | 资源不存在 |
| 3002 | `ALREADY_EXISTS` | 资源已存在 |

## 5xxx — 服务端

| 码 | 常量 | 意思 |
|----|------|------|
| 5001 | `INTERNAL_ERROR` | 服务器内部错误，看服务端日志 |

## 6xxx — 语音服务

| 码 | 常量 | 意思 | 怎么修 |
|----|------|------|--------|
| 6001 | `SFU_NOT_CONFIGURED` | 没配 SFU | 检查 `LIVEKIT_HOST` / `LIVEKIT_KEY` / `LIVEKIT_SECRET` |
| 6002 | `SFU_ERROR` | 调 LiveKit 失败 | 看 LiveKit 是不是活着、密钥对不对 |

## 7xxx — 第三方登录

| 码 | 常量 | 意思 |
|----|------|------|
| 7001 | `OAUTH_PROVIDER_NOT_FOUND` | 这个平台没配置 |
| 7002 | `OAUTH_PROVIDER_DISABLED` | 配置了但被禁用 |
| 7003 | `OAUTH_TOKEN_EXCHANGE_FAILED` | 换 token 失败，多半是 secret 不对或回调地址不匹配 |
| 7004 | `OAUTH_GET_USER_FAILED` | 拿到 token 但取不到用户信息 |

## 8xxx — 邮件

| 码 | 常量 | 意思 |
|----|------|------|
| 8001 | `EMAIL_ALREADY_EXISTS` | 邮箱已被注册 |
| 8002 | `EMAIL_CODE_INVALID` | 验证码不对 |
| 8003 | `EMAIL_CODE_EXPIRED` | 验证码过期 |
| 8004 | `EMAIL_CODE_ALREADY_USED` | 验证码用过了 |
| 8005 | `EMAIL_SEND_TOO_FREQUENT` | 发太频繁 |
| 8006 | `EMAIL_SEND_FAILED` | 发信失败，SMTP 配置有问题 |
| 8007 | `EMAIL_NOT_VERIFIED` | 邮箱未验证 |
| 8008 | `EMAIL_CODE_MAX_ATTEMPTS` | 验证码试错次数超了 |
| 8009 | `EMAIL_NOT_CONFIGURED` | 没配邮件服务 |
| 8010 | `PASSWORD_RESET_DISABLED` | 找回密码功能被关了 |

## 81xx — 存储

| 码 | 常量 | 意思 |
|----|------|------|
| 8101 | `STORAGE_NOT_CONFIGURED` | 没配存储 |
| 8102 | `STORAGE_ERROR` | 存储操作失败 |
| 8103 | `STORAGE_FILE_TOO_LARGE` | 文件太大 |
| 8104 | `STORAGE_FILE_TYPE_NOT_ALLOWED` | 文件类型不允许 |

## 前端怎么处理

一般按类别处理就够了：

```ts
switch (Math.floor(code / 1000)) {
  case 1:
    // 认证问题：1003 尝试刷新，其他跳登录
    break;
  case 2:
    // 参数问题：把 msg 显示给用户
    break;
  case 5:
    // 服务端问题：提示稍后再试
    break;
}
```

几个特殊情况要单独处理：

- `1013` 权限不足 → 提示「你没有权限」，不要跳登录
- `1015` 账号被封 → 跳到一个说明页，不要一直重试
- `1017` 限流 → 倒计时后再让用户操作

## 下一步

- [API 参考](/reference/api) — 接口列表
- [权限说明](/guide/permissions) — 1013 通常意味着什么
