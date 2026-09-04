# 域管理页房间管理功能设计

日期：2026-08-02

## 目标

在 `/manage/domains/$domainUUID` 域管理页新增「房间管理」区，实现该域下房间的完整管理：列表查看、新建、编辑、删除。当前该页面已有「域设置」与「成员管理」两栏（见 `2026-08-02-room-list-domain-detail-ui-design.md` 的布局优化）。

## 现状与约束

- 后端已具备房间 CRUD 接口（`/api/v1/room/create|list|get|update|delete`），`List` 支持 `domain_uuid` 过滤 + 分页。
- `Room` 模型带 `DomainUUID`、`CreatedBy`、`Type`(text/voice)、`Limit`、`Password`、`AudioOnly`、`AllowAudience`、`CreatedAt`。
- 前端 `api/room.ts` 仅有 `createRoom`，缺 list/update/delete。
- 权限现状漏洞：
  - `Delete` 只校验域成员身份，任意域成员可删域内任意房间。
  - `Update` 只认全局权限码 `room:update` 或创建者本人，不认域 owner/admin 对他人房间的管理权。
- 前端已复用组件：`ManagePage`/`ManageHeader`/`ManageSection`、`ConfirmModal`、`CreateRoomModal`（当前会把房间建到 `domainStore.state.currentDomainUUID`，域管理页需能指定目标域）。
- 密码在 `Update` 接口中不可修改，`Password` 字段不返回给前端。

## 设计

### 1. 后端：统一房间管理权限校验

`RoomHandler` 注入 `domainSvc *service.DomainService`，新增 helper：

```go
// 可管理某房间：全局权限码 / 域 owner・admin / 创建者本人
func (h *RoomHandler) canManageRoom(c *gin.Context, room *model.Room) bool
```

判定顺序（任一命中即放行）：

1. 当前用户全局角色持有 `room:update`（编辑）或 `room:delete`（删除）权限码，用 `h.permSvc.HasPermission`。
2. 房间属于某域（`room.DomainUUID != ""`）且 `h.domainSvc.HasDomainRole(room.DomainUUID, userUUID, service.DomainRoleAdmin)` 为真（owner 级别 4、admin 级别 3，均满足 admin 下限）。
3. `room.CreatedBy == 当前用户名`。

改动点：

- `Update`：将现有"权限码或创建者"校验替换为 `canManageRoom`（保持现有边界行为不变，仅扩展域角色放行）。
- `Delete`：在现有 `IsDomainMember` 校验后追加 `canManageRoom`，堵住任意成员删任意房间的漏洞。
- 新增/保持 `Create`、`List`、`Get` 的域成员校验不变。

`gin.go` DI 处为 `NewRoomHandler` 传入 `domainSvc`。

### 2. 前端 API 层（`api/room.ts`）

补齐：

- `listRooms(page, pageSize, domainUUID?)` → `{ rooms, total, page, size }`
- `updateRoom(req)` → 更新 name/description/limit/audio_only/allow_audience
- `deleteRoom(id)`
- `RoomRecord` 补 `created_by`、`created_at` 字段类型。

### 3. 前端组件

新建 `DomainRoomTable.tsx`（对齐 `DomainMemberTable` 风格与状态机 loading/error/empty/ready）：

- 列：名称、类型（语音/文字）、创建者、人数上限、密码（有无）、创建时间、操作。
- 分页（复用现有分页交互风格，如 `/manage/mute`）。
- 操作按钮按权限显隐：域 owner/admin 或全局 `room:update/delete` 显示「编辑/删除」；普通域成员对他人房间只读，对自己创建的房间可编辑/删除。

新建 `EditRoomModal.tsx`：编辑名称、说明、人数上限、仅语音、允许听众（不含密码）。提交走 `updateRoom`。

新建：复用 `CreateRoomModal`，为其加可选 `domainUUID` prop；域管理页传入当前域 uuid，保证新建房间归属当前域。可选地加 `onCreated` 刷新列表。

删除：复用 `ConfirmModal`，确认后 `deleteRoom` 并刷新。

### 4. 页面集成（`$domainUUID/index.tsx`）

两栏（域设置/成员管理）下方新增全宽 `ManageSection`「房间管理」：

- 顶部 actions：刷新按钮 + 「新建房间」按钮（可管理时显示）。
- 主体：`DomainRoomTable`。
- 数据源：页内 state 自管（`domainStore` 不加房间缓存），进入页面 / 切换域 / 刷新时拉取第 1 页。

## 错误处理

- 列表/编辑/删除失败：沿用页面现有 `apiErrorMessage` + `showToast` 模式，表格区展示 error 态可重试。
- 权限不足：后端返回 403 时按统一错误 toast 提示，前端不预判后端校验。

## 测试

- 后端：`room_handler_test.go` 补 Delete 权限用例（域成员删他人房间被拒、owner/admin/创建者可删）。
- 前端：`DomainRoomTable.spec.tsx` 覆盖 loading/empty/error/ready 与权限显隐；`EditRoomModal` 校验逻辑。
- 手动验证：Dev 环境建两用户，普通成员 vs owner 在域管理页的操作入口差异；新建/编辑/删除后列表刷新。

## 范围外

- 不改房间密码（后端 Update 不支持）。
- 不引入房间实时在线人数（沿用静态信息）。
- 不动平台级房间与现有域成员列表。
