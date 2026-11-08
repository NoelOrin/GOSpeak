# 文件存储

头像、聊天里的图片附件、表情图，都要有地方放。GOSpeak 支持两种放法：

- **本机磁盘**（`local`）：存在服务器上一个目录里，最简单，单机够用
- **对象存储**（`s3`）：存在 S3 兼容服务里，适合要 CDN、要多机部署的场景

默认是本机磁盘，开箱就能用。

## 本机磁盘

默认配置就这样：

```ini
STORAGE_TYPE=local
STORAGE_LOCAL_DIR=uploads
STORAGE_PATH_PREFIX=uploads/
```

文件会存到 `uploads/` 目录（相对进程工作目录）。Docker 部署时对应容器里的 `/app/uploads`，compose 已经把它挂成卷了，容器重建不会丢。

想改位置就改 `STORAGE_LOCAL_DIR`。

**什么时候该换掉它**

- 前端和后端不在同一台机器上，靠 Nginx 分流静态文件
- 想上 CDN
- 想部署多个后端实例做负载均衡（多实例之间文件不共享，这个方案会挂）

## 对象存储

支持任何 S3 兼容服务：AWS S3、Cloudflare R2、MinIO、RustFS、阿里云 OSS（S3 兼容模式）等。

```ini
STORAGE_TYPE=s3
STORAGE_ENDPOINT=https://your-s3-endpoint
STORAGE_BUCKET=gospeak
STORAGE_REGION=auto
STORAGE_ACCESS_KEY=你的-key
STORAGE_SECRET_KEY=你的-secret
STORAGE_PATH_PREFIX=uploads/
STORAGE_PUBLIC_BASE_URL=https://cdn.example.com
STORAGE_ENCRYPT_KEY=<64 位十六进制>
```

几个要点：

- **`STORAGE_REGION`**：R2 填 `auto`，AWS 填实际区域，MinIO 随便填但要非空
- **`STORAGE_PUBLIC_BASE_URL`**：走 CDN 时填 CDN 域名，不填就用 endpoint 直连
- **`STORAGE_ENCRYPT_KEY`**：生产必填，用来加密存在数据库里的 Access Key / Secret。丢了这串密钥，之前存的配置就解不开了

### 用 Docker Compose 自带的 RustFS

compose 里有个 `rustfs` 档位，是 S3 兼容的本地对象存储：

```ini
COMPOSE_PROFILES=postgres,livekit,rustfs,app
STORAGE_TYPE=s3
STORAGE_ENDPOINT=http://rustfs:9000
STORAGE_BUCKET=gospeak
STORAGE_ACCESS_KEY=rustfsadmin
STORAGE_SECRET_KEY=rustfsadmin
STORAGE_REGION=us-east-1
```

生产环境记得改掉默认的 `rustfsadmin`。

## 上传是怎么走的

前端不走「先传后端再转存」，而是直接传对象存储：

```
前端                    后端                    存储
 │                       │                       │
 ├─ POST /storage/presign ─►                     │
 │◄──── 预签名 URL ───────┤                       │
 │                                               │
 ├─────────── PUT 文件（直传）──────────────────► │
 │                                               │
 ├─ POST /storage/confirm ─►                     │
 │                       ├── 确认对象存在 ───────►│
 │◄──── 文件 URL ────────┤                       │
```

本机磁盘模式下前端也是先拿预签名地址，只是这个地址指向后端自己的一个上传端点。

好处是大文件不经过业务逻辑，后端不会被上传流量拖慢。

## 在后台改配置

管理员可以在「管理 → 存储」里直接改，改完存数据库，立即生效，不用重启。

改完有个「测试配置」按钮，会真的往存储里写一个测试对象再删掉，验证连通性。

数据库里的配置优先级高于环境变量。想回到环境变量的值，去后台清空。

## 排查问题

**上传报 500，日志说 `init s3 provider failed`**
端点地址或密钥错了。先用 `curl` 直接测端点通不通：

```bash
curl -I https://your-s3-endpoint
```

**上传成功但图片打不开**
`STORAGE_PUBLIC_BASE_URL` 配错了，或者桶没开公共读。用后台的「测试配置」能看出问题。

**换存储后老图片全裂了**
换存储不会迁移已有文件。老文件还在原来的地方，要么手动搬，要么保留旧配置。

**多实例部署后有的图片能看有的不能**
用了本机磁盘但没做共享存储。换成 S3。

## 相关接口

| 方法 | 路径 | 权限 | 用途 |
|------|------|------|------|
| POST | `/api/v1/storage/presign` | 登录即可 | 拿上传地址 |
| POST | `/api/v1/storage/confirm` | 登录即可 | 确认上传完成 |
| POST | `/api/v1/storage/upload` | 登录即可 | 直传（小文件） |
| POST | `/api/v1/storage/delete` | `storage:delete` | 删对象 |
| POST | `/api/v1/storage/config` | `storage:read` | 读配置 |
| POST | `/api/v1/storage/update-config` | `storage:manage` | 改配置 |
| POST | `/api/v1/storage/test-config` | `storage:read` | 测连通性 |

## 下一步

- [配置项](/guide/configuration) — 全部存储相关变量
- [部署到服务器](/deploy/) — 生产环境的存储选型建议
