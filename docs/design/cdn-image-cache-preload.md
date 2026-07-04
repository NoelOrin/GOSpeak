# CDN 图片缓存与主动预热方案

> **项目**: GoRTC  
> **日期**: 2026-06-29  
> **状态**: 方案设计  
> **适用范围**: 图片上传、静态资源访问、CDN 缓存、缓存预热与刷新

## 1. 目标

让上传后的图片尽快进入 CDN 缓存，减少用户首次访问图片时的回源延迟，并降低源站压力。

目标包括：

1. 图片统一通过 CDN 域名访问
2. 源站返回明确的缓存响应头
3. 上传完成后主动触发 CDN 预热
4. 图片更新时能正确刷新旧缓存
5. 能通过响应头判断 CDN 是否命中缓存

## 2. 基本原理

CDN 缓存图片通常有两种方式：

| 方式 | 说明 | 适用场景 |
|------|------|----------|
| 被动缓存 | 用户第一次访问 CDN URL，CDN 回源拉取并缓存 | 普通静态资源，允许首次访问稍慢 |
| 主动预热 | 上传后调用 CDN 预热接口，让 CDN 提前回源拉取 | 头像、封面、房间图片等需要首访更快的资源 |

推荐使用“源站缓存头 + CDN 缓存规则 + 上传后主动预热”的组合方案。

```text
用户上传图片
    ↓
后端保存图片到源站或对象存储
    ↓
生成 CDN 图片 URL
    ↓
异步任务调用 CDN 预热接口
    ↓
CDN 主动回源拉取图片并缓存
    ↓
用户访问 CDN URL，优先命中缓存
```

## 3. URL 设计

图片应使用稳定的静态路径，不建议通过业务接口代理图片内容。

推荐：

```text
https://cdn.example.com/uploads/avatars/2026/06/user-a8f3c1d9.webp
https://cdn.example.com/uploads/rooms/2026/06/room-cover-91b2e8aa.png
```

不推荐：

```text
https://api.example.com/api/v1/image?id=123
```

原因：

1. 静态路径更容易被 CDN 识别和缓存
2. 文件后缀可以直接匹配 CDN 缓存规则
3. URL 中包含 hash 后，可以长期缓存，避免覆盖更新带来的旧缓存问题

## 4. 源站缓存头

源站必须明确允许 CDN 缓存图片。推荐响应头：

```http
Cache-Control: public, max-age=31536000, immutable
```

含义：

| 指令 | 说明 |
|------|------|
| `public` | 允许 CDN、浏览器等共享缓存保存响应 |
| `max-age=31536000` | 缓存 365 天 |
| `immutable` | URL 内容不会变化，缓存期内无需重新校验 |

如果图片 URL 会被覆盖更新，不要使用 `immutable`，并降低缓存时间：

```http
Cache-Control: public, max-age=86400
```

### 4.1 Nginx 示例

```nginx
location /uploads/ {
    root /var/www;
    expires 365d;
    add_header Cache-Control "public, max-age=31536000, immutable";
}
```

如果使用对象存储，需要在对象元数据或 Bucket 静态资源规则中设置同等的 `Cache-Control`。

## 5. CDN 缓存规则

CDN 控制台中建议配置：

| 配置项 | 推荐值 |
|--------|--------|
| 回源地址 | 源站域名或对象存储源站域名 |
| 缓存路径 | `/uploads/*` |
| 缓存后缀 | `jpg,jpeg,png,webp,gif,svg,avif` |
| 缓存时间 | 30 天到 365 天 |
| Query String | 如果使用文件名 hash，建议忽略；如果靠 `?v=` 版本号，必须参与缓存键 |

优先使用文件名 hash，而不是 query 参数版本号：

```text
/uploads/avatars/user-a8f3c1d9.webp
```

比下面这种更稳定：

```text
/uploads/avatars/user.webp?v=20260629
```

原因是不同 CDN 对 query string 的默认缓存策略不一致，文件名 hash 更可控。

## 6. 主动预热流程

上传成功后，后端不应同步等待 CDN 预热完成再返回给用户。推荐把预热放到异步任务中。

```text
POST /api/v1/upload/image
    ↓
保存图片
    ↓
生成图片 CDN URL
    ↓
写入业务数据
    ↓
投递 cdn_prefetch_job
    ↓
立即返回图片 URL
```

异步任务处理：

```text
cdn_prefetch_job
    ↓
调用 CDN 服务商预热接口
    ↓
记录预热结果
    ↓
失败时按策略重试
```

### 6.1 服务商接口示例

不同服务商的接口名称不同：

| 服务商 | 常见能力 |
|--------|----------|
| 阿里云 CDN | `PushObjectCache` |
| 腾讯云 CDN | `PushUrlsCache` |
| 七牛云 | `prefetch` |
| Cloudflare | 常规 CDN 没有完全等价的全网预热接口，通常结合主动请求、Cache Reserve 或 Workers 策略 |

预热接口通常接收一个或多个完整 URL：

```json
{
  "urls": [
    "https://cdn.example.com/uploads/avatars/2026/06/user-a8f3c1d9.webp"
  ]
}
```

## 7. 没有预热接口时的兜底方案

如果 CDN 没有提供官方预热接口，可以主动请求 CDN URL：

```bash
curl -s -o /dev/null https://cdn.example.com/uploads/avatars/2026/06/user-a8f3c1d9.webp
```

或只请求响应头：

```bash
curl -I https://cdn.example.com/uploads/avatars/2026/06/user-a8f3c1d9.webp
```

注意：这种方式通常只会预热请求命中的边缘节点，不一定能覆盖全网 CDN 节点。官方预热接口更可靠。

## 8. 图片更新策略

### 8.1 推荐策略：新文件名

图片内容变化时，生成新的文件名：

```text
旧 URL: /uploads/avatars/user-a8f3c1d9.webp
新 URL: /uploads/avatars/user-f4c7a912.webp
```

优点：

1. 不需要刷新旧缓存
2. 可以使用长期缓存
3. 浏览器和 CDN 都不会拿到旧内容

### 8.2 兼容策略：刷新旧 URL

如果业务必须复用同一个 URL，例如：

```text
/uploads/avatars/user-123.webp
```

更新流程必须是：

```text
1. 覆盖源站图片
2. 调用 CDN Refresh/Purge 删除旧缓存
3. 调用 CDN Preload/Prefetch 预热新内容
```

不刷新缓存就覆盖源站，会导致用户继续看到旧图，直到 CDN 缓存过期。

## 9. 验证方式

使用 `curl -I` 查看 CDN 响应头：

```bash
curl -I https://cdn.example.com/uploads/avatars/2026/06/user-a8f3c1d9.webp
```

重点关注：

```http
Cache-Control: public, max-age=31536000, immutable
Age: 12345
X-Cache: HIT
```

常见字段说明：

| 字段 | 说明 |
|------|------|
| `Cache-Control` | 源站或 CDN 返回的缓存策略 |
| `Age` | 资源已在缓存中存在的秒数，存在且递增通常说明命中缓存 |
| `X-Cache: HIT` | CDN 命中缓存 |
| `X-Cache: MISS` | CDN 未命中，已回源拉取 |
| `CF-Cache-Status: HIT` | Cloudflare 命中缓存 |

不同 CDN 的命中字段名称不同，应以服务商文档为准。

## 10. 后端落地建议

后端可以抽象一个 CDN 服务接口，业务层只关心预热和刷新动作，不直接绑定具体服务商。

```go
type CDNProvider interface {
    Prefetch(urls []string) error
    Purge(urls []string) error
}
```

上传服务中只投递任务：

```text
uploadService.SaveImage(file)
    ↓
imageURL := buildCDNURL(path)
    ↓
cdnJob.EnqueuePrefetch(imageURL)
    ↓
return imageURL
```

这样后续从阿里云切换到腾讯云、七牛或 Cloudflare 时，不需要改上传业务逻辑。

## 11. 最小可行方案

第一阶段可以先做以下内容：

1. 图片 URL 统一改为 CDN 域名
2. `/uploads/*` 源站返回 `Cache-Control: public, max-age=31536000, immutable`
3. CDN 控制台配置图片后缀缓存规则
4. 上传完成后异步调用 CDN 预热接口
5. 使用文件名 hash 避免覆盖更新
6. 通过 `curl -I` 检查 `HIT/MISS` 和 `Age`

如果暂时没有队列系统，可以先在上传成功后启动后台任务执行预热，但不要阻塞上传接口响应。
