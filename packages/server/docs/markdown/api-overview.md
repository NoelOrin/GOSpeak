# GoRTC API 总览

> 版本: 1.0 | 基础路径: `/api/v1`

## 服务信息

| 项目 | 内容 |
|------|------|
| 标题 | GoRTC API |
| 描述 | GoRTC - WebRTC Server API |
| 版本 | 1.0 |
| 服务条款 | https://github.com/NoelOrin/GoRTC |
| 许可证 | MIT (https://opensource.org/licenses/MIT) |
| 联系人 | NoelOrin (https://github.com/NoelOrin) |

## 基础 URL

**开发环境:** `http://localhost:8098`
**基础路径:** `/api/v1`

## 鉴权方式

所有需要登录的接口使用 **Bearer Token** 鉴权：

```
Authorization: Bearer <your-jwt-token>
```

### 获取 Token

1. 调用 `POST /api/v1/auth/login` 获取 `token` 和 `refresh_token`
2. 在 Swagger UI 中点击 **Authorize** 按钮，输入 `Bearer <token>`
3. Token 过期后使用 `POST /api/v1/auth/refresh_token` 刷新

## 通用响应格式

所有接口返回统一的 JSON 结构：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

### 错误码

| code | 名称 | 说明 |
|------|------|------|
| 0 | SUCCESS | 请求成功 |
| 1 | ERROR | 通用错误 |
| 1001 | TOKEN_NOT_EXIST | Token 不存在 |
| 1002 | TOKEN_WRONG | Token 无效 |
| 1003 | TOKEN_RUNTIME | Token 过期 |
| 2001 | INVALID_PARAMS | 请求参数错误 |
| 2002 | UNAUTHORIZED | 未授权 |
| 2003 | NOT_FOUND | 资源不存在 |
| 5001 | INTERNAL_ERROR | 服务器内部错误 |

## 接口列表

| 模块 | 路径 | 方法 | 需鉴权 |
|------|------|------|--------|
| Auth | `/auth/login` | POST | ❌ |
| Auth | `/auth/register` | POST | ❌ |
| Auth | `/auth/refresh_token` | POST | ❌ |
| Auth | `/auth/logout` | POST | ✅ |
| Auth | `/auth/refresh` | POST | ✅ |
| Signal | `/signal/token` | POST | ❌ |
| Signal | `/signal/signal` | POST | ❌ |
| Signal | `/signal/rooms` | GET | ❌ |
| Signal | `/signal/participants` | GET | ❌ |
| User | `/user/profile` | GET | ✅ |
| User | `/user/list` | GET | ✅ |
| User | `/user/{id}` | GET | ✅ |
| User | `/user/{id}` | DELETE | ✅ |