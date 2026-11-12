# 文件存储

头像、聊天中的图片附件、表情图都需要存放位置。GOSpeak 支持两种存储方式：

- **本机磁盘**（`local`）：存放在服务器的一个目录里，配置最简单，单机部署足够使用
- **对象存储**（`s3`）：存放在 S3 兼容服务里，适合需要 CDN 或多机部署的场景

默认为本机磁盘，开箱即可使用。

## 本机磁盘

默认配置如下：

```ini
STORAGE_TYPE=local
STORAGE_LOCAL_DIR=uploads
STORAGE_PATH_PREFIX=uploads/
```

文件会存到 `uploads/` 目录（相对进程工作目录）。Docker 部署时对应容器内的 `/app/uploads`，compose 已将其挂载为卷，容器重建不会丢失数据。

如需调整位置，修改 `STORAGE_LOCAL_DIR` 即可。

**何时需要更换存储方式**

- 前端和后端不在同一台机器上，依靠 Nginx 分流静态文件
- 需要接入 CDN
- 需要部署多个后端实例做负载均衡（多实例之间文件不共享，本机磁盘方案在此场景下不可用）

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

- **`STORAGE_REGION`**：R2 填 `auto`，AWS 填实际区域，MinIO 取值任意但必须非空
- **`STORAGE_PUBLIC_BASE_URL`**：走 CDN 时填 CDN 域名，不填则通过 endpoint 直连
- **`STORAGE_ENCRYPT_KEY`**：生产环境必填，用于加密存入数据库的 Access Key / Secret。该密钥丢失后，之前存储的配置将无法解密

### 用 Docker Compose 自带的 RustFS

compose 里有一个 `rustfs` 档位，提供 S3 兼容的本地对象存储：

```ini
COMPOSE_PROFILES=app,livekit,postgres,rustfs
STORAGE_TYPE=s3
STORAGE_ENDPOINT=http://rustfs:9000
STORAGE_BUCKET=gospeak
STORAGE_ACCESS_KEY=rustfsadmin
STORAGE_SECRET_KEY=rustfsadmin
STORAGE_REGION=us-east-1
```

生产环境应替换默认的 `rustfsadmin` 凭据。

## 上传流程

前端不采用「先传后端再转存」的方式，而是直接上传到对象存储：

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

本机磁盘模式下前端同样先获取预签名地址，只是该地址指向后端自身的上传端点。

这种设计的好处是大文件不经过业务逻辑，后端不会被上传流量拖慢。

## 在后台改配置

管理员可以在「管理 → 存储」页面直接修改，修改结果存入数据库，立即生效，无需重启。

保存后有「测试配置」按钮，会实际向存储写入一个测试对象再删除，用于验证连通性。

数据库中的配置优先级高于环境变量。需要回到环境变量的取值时，在后台清空对应配置项。

## 排查问题

**上传报 500，日志显示 `init s3 provider failed`**
端点地址或密钥有误。先用 `curl` 直接测试端点连通性：

```bash
curl -I https://your-s3-endpoint
```

**上传成功但图片无法打开**
`STORAGE_PUBLIC_BASE_URL` 配置有误，或桶未开启公共读。使用后台的「测试配置」可以定位问题。

**更换存储后历史图片全部失效**
更换存储不会迁移已有文件。旧文件仍保留在原位置，需要手动迁移，或保留旧配置。

**多实例部署后部分图片无法访问**
使用了本机磁盘但未配置共享存储，应改用 S3。

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
