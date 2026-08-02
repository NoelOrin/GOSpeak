# 域管理页房间管理功能实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `/manage/domains/$domainUUID` 域管理页新增「房间管理」区块，实现该域下房间的列表查看、新建、编辑、删除，并修复后端 Delete 任意域成员可删任意房间的权限漏洞。

**Architecture:** 后端 `RoomHandler` 注入 `domainSvc`，新增 `canManageRoom` helper 统一 Update/Delete 的房间管理权限判定（全局权限码 → 域 owner/admin → 创建者本人）。前端补齐 `api/room.ts` 的 list/update/delete 接口，新增 `DomainRoomTable`（对齐 `DomainMemberTable` 状态机）与 `EditRoomModal`，复用 `CreateRoomModal`（加可选 `domainUUID` prop），页面内 state 自管房间列表并接入分页。

**Tech Stack:** Go 1.26 · Gin · GORM · SolidJS · TypeScript · TanStack Solid Router · Tailwind v4 · daisyUI · lucide-solid · Vitest

## Global Constraints

- 后端 `RoomHandler` 构造签名变为 `NewRoomHandler(roomSvc, permSvc, domainSvc)`；`canManageRoom` 判定顺序不可改变：全局权限码 → 域 owner/admin（`HasDomainRole(domainUUID, userUUID, service.DomainRoleAdmin)`）→ `room.CreatedBy == c.Get("username")`；`permSvc`/`domainSvc` 必须 nil 安全。
- `Delete` 在既有 `IsDomainMember` 校验之后追加 `canManageRoom`；`Create`/`List`/`Get` 的域成员校验保持不变。
- 前端 `RoomRecord` 按后端 snake_case：新增 `created_at?: string`，删除从未使用的 camelCase `createdAt?`/`updatedAt?` 字段。
- 房间表格**不设**「密码(有无)」列（后端 `Room.Password` 带 `json:"-"` 永不返回）。
- 分页复用 discover 页面样式：上一页/下一页按钮 + `{page} / {totalPages}` 文本。
- 前端房间操作按钮显隐规则：`room.created_by === currentUser()?.name` 或域 owner/admin 或全局 `room:update`/`room:delete` 任一命中显示「编辑/删除」；普通域成员对他人房间只读。
- 后端 403 → 前端统一 `apiErrorMessage` + `showToast` 错误提示；表格区 error 态可重试。
- 工作树 `app/web/src/components/room/roomList.tsx` 存在**未提交**改动（先前计划遗留）——本次各任务 commit 只暂存各自文件，禁止 `git add .` 或纳入 roomList.tsx。
- 代码中不加非必要注释，不使用 emoji。Go 文件 snake_case、类型 PascalCase；前端组件名 PascalCase。
- 手动验证（多账号权限差异）由 agent 执行时，结果以 Markdown 存入 `agent_test_logs/`，命名 `{内容}-{时间}.md`。

---

### Task 1: 后端 canManageRoom 权限校验 + Delete/Update 重新接线 + DI + 测试

**Files:**
- Modify: `app/server/internal/handler/room_handler.go`
- Modify: `app/server/internal/handler/room_handler_test.go`
- Modify: `app/server/server/gin.go`

**Interfaces:**
- Consumes: `h.permSvc.HasPermission(roleName, permCode string) bool`、`h.domainSvc.HasDomainRole(domainUUID, userUUID, minRole string) bool`、`permcode.PermRoomUpdate/PermRoomDelete`、`middleware.IsDomainMember(domainUUID, userUUID) bool`、`currentUserUUID(c)`、`pkg.Fail(c, pkg.FORBIDDEN, msg)`。
- Produces: `NewRoomHandler(roomSvc *service.RoomService, permSvc *service.PermissionService, domainSvc *service.DomainService) *RoomHandler`；`(h *RoomHandler) canManageRoom(c *gin.Context, room *model.Room, perm string) bool`。后续任务不依赖本任务产物，但全仓编译依赖签名一致性。

- [ ] **Step 1: 写失败测试**

在 `app/server/internal/handler/room_handler_test.go` 文件末尾追加以下测试。该测试引用新的三参构造函数 `NewRoomHandler(svc, nil, domainSvc)`，目前无法编译（红）。

```go
func TestRoomHandler_Delete_RequiresManagePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	middleware.SetDomainChecker(func(domainUUID, userUUID string) bool { return true })
	t.Cleanup(func() { middleware.SetDomainChecker(nil) })

	newEnv := func(t *testing.T, roleName, requesterUUID, createdBy string) *gin.Engine {
		t.Helper()
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		if err := db.AutoMigrate(&model.Room{}, &model.Domain{}, &model.DomainMember{}); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		g := &model.Domain{Name: "Test", OwnerUUID: "owner-1"}
		if err := db.Create(g).Error; err != nil {
			t.Fatalf("seed domain: %v", err)
		}
		if err := db.Create(&model.DomainMember{DomainUUID: g.UUID, UserUUID: requesterUUID, RoleName: roleName}).Error; err != nil {
			t.Fatalf("seed member: %v", err)
		}
		room := model.Room{Name: "lobby", DomainUUID: g.UUID, CreatedBy: createdBy}
		if err := db.Create(&room).Error; err != nil {
			t.Fatalf("seed room: %v", err)
		}
		domainSvc := service.NewDomainService(repository.NewDomainRepository(db))
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("username", requesterUUID)
			c.Set("user_uuid", requesterUUID)
			c.Set("role", "user")
			c.Next()
		})
		h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, domainSvc)
		r.POST("/api/v1/room/delete", h.Delete)
		return r
	}

	performDelete := func(r *gin.Engine) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/room/delete", strings.NewReader(`{"id":1}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("普通域成员不能删除他人创建的房间", func(t *testing.T) {
		w := performDelete(newEnv(t, service.DomainRoleMember, "member-1", "creator-1"))
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if code := intCode(resp["code"]); code != 1013 {
			t.Fatalf("expected 1013 for member delete, got %d: %s", code, resp["msg"])
		}
	})

	t.Run("域 owner 可以删除他人创建的房间", func(t *testing.T) {
		w := performDelete(newEnv(t, service.DomainRoleOwner, "owner-1", "creator-1"))
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if code := intCode(resp["code"]); code != 0 {
			t.Fatalf("expected 0 for owner delete, got %d: %s", code, resp["msg"])
		}
	})

	t.Run("域 admin 可以删除他人创建的房间", func(t *testing.T) {
		w := performDelete(newEnv(t, service.DomainRoleAdmin, "admin-1", "creator-1"))
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if code := intCode(resp["code"]); code != 0 {
			t.Fatalf("expected 0 for admin delete, got %d: %s", code, resp["msg"])
		}
	})

	t.Run("创建者本人可以删除自己创建的房间", func(t *testing.T) {
		w := performDelete(newEnv(t, service.DomainRoleMember, "creator-1", "creator-1"))
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if code := intCode(resp["code"]); code != 0 {
			t.Fatalf("expected 0 for creator delete, got %d: %s", code, resp["msg"])
		}
	})
}
```

注：`intCode` 定义于同包 `domain_handler_test.go:95`，无需新增。新测试只使用 `room_handler_test.go` 已有 import（encoding/json、net/http、net/http/httptest、strings、testing、middleware、model、repository、service、gin、sqlite、gorm）。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/server && go test ./internal/handler/ -run TestRoomHandler_Delete_RequiresManagePermission -count=1`
Expected: 编译失败 `not enough arguments in call to NewRoomHandler`（红）。

- [ ] **Step 3: 实现 canManageRoom + 构造签名 + Update/Delete 接线**

修改 `app/server/internal/handler/room_handler.go`：

3a. 结构体与构造函数（第 13-20 行）替换为：

```go
type RoomHandler struct {
	roomSvc   *service.RoomService
	permSvc   *service.PermissionService
	domainSvc *service.DomainService
}

func NewRoomHandler(roomSvc *service.RoomService, permSvc *service.PermissionService, domainSvc *service.DomainService) *RoomHandler {
	return &RoomHandler{roomSvc: roomSvc, permSvc: permSvc, domainSvc: domainSvc}
}
```

3b. 在 `currentUserUUID` 函数（第 34 行 `}`）之后追加：

```go
func (h *RoomHandler) canManageRoom(c *gin.Context, room *model.Room, perm string) bool {
	username, _ := c.Get("username")
	usernameStr, _ := username.(string)
	if h.permSvc != nil && h.permSvc.HasPermission(roleFromContext(c), perm) {
		return true
	}
	if room.DomainUUID != "" && h.domainSvc != nil &&
		h.domainSvc.HasDomainRole(room.DomainUUID, currentUserUUID(c), service.DomainRoleAdmin) {
		return true
	}
	return room.CreatedBy == usernameStr
}

func roleFromContext(c *gin.Context) string {
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	return roleStr
}
```

3c. `Update` 权限块（原第 217-235 行 `// 资源归属校验：...` 注释开始至对应的 `}` 结束）整体替换为：

```go
	if !h.canManageRoom(c, room, permcode.PermRoomUpdate) {
		pkg.Fail(c, pkg.FORBIDDEN, "没有权限编辑该房间")
		return
	}
```

3d. `Delete`（第 286-289 行 IsDomainMember 校验块之后、`if err := h.roomSvc.Delete(req.ID); err != nil {` 之前）追加：

```go
	if !h.canManageRoom(c, room, permcode.PermRoomDelete) {
		pkg.Fail(c, pkg.FORBIDDEN, "没有权限删除该房间")
		return
	}
```

3e. 更新测试文件 `room_handler_test.go` 中全部 6 处调用点（第 46、94、136、183、218、258 行）：

```go
h := NewRoomHandler(service.NewRoomService(repository.NewRoomRepository(db)), nil, nil)
```

3f. 修改 `app/server/server/gin.go` DI 接线：把 `domainSvc` 构造及注入（现第 329-331 行）移到 `roomH`（现第 325 行）之前，并把 roomH 改为三参：

```go
	domainSvc := service.NewDomainService(repository.NewDomainRepository(repository.DB))
	middleware.SetDomainChecker(domainSvc.IsMember)
	signalHub.SetDomainChecker(domainSvc.IsMember)
```

```go
	roomH := handler.NewRoomHandler(roomSvc, permSvc, domainSvc)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/server && go test ./internal/handler/ -count=1`
Expected: 全部 PASS（含新增 4 个 Delete 权限用例与既有 6 个测试）。
Run: `cd app/server && go build ./...`
Expected: 退出码 0，无编译错误。

- [ ] **Step 5: 提交**

```bash
git add app/server/internal/handler/room_handler.go app/server/internal/handler/room_handler_test.go app/server/server/gin.go
git commit -m "feat(server): enforce room manage permission on update and delete"
```

---

### Task 2: 前端 api/room.ts 补齐 list/update/delete

**Files:**
- Modify: `app/web/src/api/room.ts`

**Interfaces:**
- Produces: `listRooms(page: number, pageSize: number, domainUUID?: string) => Promise<RoomListResult>`、`updateRoom(req: UpdateRoomReq) => Promise<RoomRecord>`、`deleteRoom(id: number) => Promise<void>`、`RoomListResult { rooms, total, page, size }`、`UpdateRoomReq { id, name?, description?, limit?, audio_only?, allow_audience? }`、`RoomRecord` 增 `created_at?: string` 删 `createdAt?`/`updatedAt?`。
- Consumes: `apiClient.post`、`Result<T>`。后续 Task 4（EditRoomModal 用 updateRoom）、Task 5（DomainRoomTable 用 RoomRecord/listRooms）、Task 6（页面用 listRooms/deleteRoom）依赖本任务。

- [ ] **Step 1: 重写 api/room.ts**

将 `app/web/src/api/room.ts` 整体替换为：

```ts
import type { AxiosResponse } from "axios";
import type { Result } from "./apiClient";
import apiClient from "./apiClient";

export interface CreateRoomReq {
	name: string;
	password?: string;
	description: string;
	limit: number;
	audio_only: boolean;
	allow_audience: boolean;
	type?: "text" | "voice";
	domain_uuid?: string;
}

export interface RoomRecord {
	id: number;
	uuid: string;
	name: string;
	password?: string;
	description: string;
	limit: number;
	audio_only: boolean;
	allow_audience: boolean;
	type?: "text" | "voice";
	created_by?: string;
	created_at?: string;
	domain_uuid?: string;
}

export interface RoomListResult {
	rooms: RoomRecord[];
	total: number;
	page: number;
	size: number;
}

export interface UpdateRoomReq {
	id: number;
	name?: string;
	description?: string;
	limit?: number;
	audio_only?: boolean;
	allow_audience?: boolean;
}

export async function createRoom(req: CreateRoomReq): Promise<RoomRecord> {
	const res = (await apiClient.post({
		url: "/api/v1/room/create",
		data: req,
	})) as AxiosResponse<Result<RoomRecord>>;

	if (!(res as any).data.data) throw new Error("room data is missing");
	return (res as any).data.data;
}

export async function listRooms(
	page: number,
	pageSize: number,
	domainUUID?: string,
): Promise<RoomListResult> {
	const res = (await apiClient.post({
		url: "/api/v1/room/list",
		data: { page, page_size: pageSize, domain_uuid: domainUUID },
	})) as AxiosResponse<Result<RoomListResult>>;

	if (!(res as any).data.data) throw new Error("room data is missing");
	return (res as any).data.data;
}

export async function updateRoom(req: UpdateRoomReq): Promise<RoomRecord> {
	const res = (await apiClient.post({
		url: "/api/v1/room/update",
		data: req,
	})) as AxiosResponse<Result<RoomRecord>>;

	if (!(res as any).data.data) throw new Error("room data is missing");
	return (res as any).data.data;
}

export async function deleteRoom(id: number): Promise<void> {
	await apiClient.post({ url: "/api/v1/room/delete", data: { id } });
}
```

- [ ] **Step 2: 类型与格式检查**

Run: `cd app/web && pnpm exec tsc --noEmit`
Expected: 退出码 0（删除 camelCase 字段不产生任何引用错误；已确认全仓无 `RoomRecord.createdAt/updatedAt` 使用者）。
Run: `cd app/web && pnpm exec biome check`
Expected: 退出码 0；如报格式漂移先 `pnpm exec biome format --write src/api/room.ts` 再复跑。

- [ ] **Step 3: 提交**

```bash
git add app/web/src/api/room.ts
git commit -m "feat(web): add room list update delete api"
```

---

### Task 3: CreateRoomModal 支持指定目标域

**Files:**
- Modify: `app/web/src/components/modal/createRoomModal.tsx`

**Interfaces:**
- Consumes: `CreateRoomConfig.domainUUID`（已存在，第 19 行）。
- Produces: `CreateRoomModalProps` 新增可选 `domainUUID?: string`；payload 的 `domainUUID` 改为 `props.domainUUID ?? domainStore.state.currentDomainUUID ?? undefined`。既有调用方 roomList.tsx / quick-actions.tsx 不传该 prop，行为不变。Task 6 依赖本任务。

- [ ] **Step 1: 加 prop 并改 payload**

修改 `app/web/src/components/modal/createRoomModal.tsx`：

1a. Props 接口（第 22-26 行）追加字段：

```tsx
interface CreateRoomModalProps {
	ref: HTMLDialogElement;
	onClose: () => void;
	domainUUID?: string;
	onCreated?: (config: CreateRoomConfig) => void | Promise<void>;
}
```

1b. payload 的 `domainUUID` 行（第 66 行）替换为：

```tsx
					domainUUID: props.domainUUID ?? domainStore.state.currentDomainUUID ?? undefined,
```

- [ ] **Step 2: 类型与格式检查**

Run: `cd app/web && pnpm exec tsc --noEmit`
Expected: 退出码 0。
Run: `cd app/web && pnpm exec biome check`
Expected: 退出码 0；格式漂移先 format 再复跑。

- [ ] **Step 3: 提交**

```bash
git add app/web/src/components/modal/createRoomModal.tsx
git commit -m "feat(web): support target domain in create room modal"
```

---

### Task 4: 新增 EditRoomModal

**Files:**
- Create: `app/web/src/components/modal/editRoomModal.tsx`

**Interfaces:**
- Consumes: `updateRoom`/`RoomRecord`（Task 2）、`Form`/`FormFieldConfig`（`@/components/form`）、`createForm`（`@tanstack/solid-form`）、`showToast`（solid-notifications）。
- Produces: `EditRoomModal`（props: `{ ref: HTMLDialogElement; room: RoomRecord; onClose: () => void; onSaved: (room: RoomRecord) => void | Promise<void> }`）、`validateEditRoomForm(name: string, description: string, limit: number | "") => EditRoomFormErrors`。Task 5 的 spec 与 Task 6 页面集成依赖本任务。

- [ ] **Step 1: 写校验逻辑测试（红）**

创建 `app/web/src/components/domain/DomainRoomTable.spec.tsx`，先只写校验测试（编辑/删除/状态机用例在 Task 5 追加；本步运行会因 `validateEditRoomForm` 未定义而失败）：

```tsx
import { describe, expect, it } from "vitest";
import { validateEditRoomForm } from "../modal/EditRoomModal";

describe("EditRoomModal form validation", () => {
	it("rejects an empty room name", () => {
		expect(validateEditRoomForm("   ", "desc", 12)).toEqual({
			name: "房间名称至少需要 2 个字符",
		});
	});

	it("rejects limit below two", () => {
		expect(validateEditRoomForm("lobby", "desc", 1)).toEqual({
			limit: "人数上限至少为 2",
		});
	});

	it("rejects empty limit", () => {
		expect(validateEditRoomForm("lobby", "desc", "")).toEqual({
			limit: "人数上限至少为 2",
		});
	});

	it("rejects description over 120 characters", () => {
		expect(validateEditRoomForm("lobby", "x".repeat(121), 12)).toEqual({
			description: "房间说明不能超过 120 个字符",
		});
	});

	it("accepts a valid room config", () => {
		expect(validateEditRoomForm("lobby", "desc", 12)).toEqual({});
	});
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd app/web && pnpm exec vitest run src/components/domain/DomainRoomTable.spec.tsx`
Expected: FAIL（`validateEditRoomForm` 不存在，Cannot find module）。

- [ ] **Step 3: 实现 EditRoomModal**

创建 `app/web/src/components/modal/editRoomModal.tsx`：

```tsx
import { createForm } from "@tanstack/solid-form";
import { type Component, Show } from "solid-js";
import { showToast } from "solid-notifications";
import type { RoomRecord } from "@/api/room";
import { updateRoom } from "@/api/room";
import { Form, type FormFieldConfig } from "@/components/form";

export interface EditRoomFormErrors {
	name?: string;
	description?: string;
	limit?: string;
}

export function validateEditRoomForm(
	name: string,
	description: string,
	limit: number | "",
): EditRoomFormErrors {
	const errors: EditRoomFormErrors = {};
	const trimmed = name.trim();
	if (trimmed.length < 2) errors.name = "房间名称至少需要 2 个字符";
	else if (trimmed.length > 32) errors.name = "房间名称不能超过 32 个字符";
	if (description.trim().length > 120)
		errors.description = "房间说明不能超过 120 个字符";
	if (limit === "" || Number.isNaN(limit) || limit < 2)
		errors.limit = "人数上限至少为 2";
	else if (limit > 200) errors.limit = "人数上限不能超过 200";
	return errors;
}

interface EditRoomModalProps {
	ref: HTMLDialogElement;
	room: RoomRecord;
	onClose: () => void;
	onSaved: (room: RoomRecord) => void | Promise<void>;
}

const EditRoomModal: Component<EditRoomModalProps> = (props) => {
	const roomNameValidation = (value: string) => {
		const trimmed = value.trim();
		if (trimmed.length < 2) return "房间名称至少需要 2 个字符";
		if (trimmed.length > 32) return "房间名称不能超过 32 个字符";
		return undefined;
	};

	const limitValidation = (value: number) => {
		if (Number.isNaN(value)) return "请填写人数上限";
		if (value < 2) return "人数上限至少为 2";
		if (value > 200) return "人数上限不能超过 200";
		return undefined;
	};

	const form = createForm(() => ({
		defaultValues: {
			name: props.room.name,
			description: props.room.description,
			limit: props.room.limit,
			audioOnly: props.room.audio_only,
			allowAudience: props.room.allow_audience,
		},
		onSubmit: async ({ value }) => {
			try {
				const updated = await updateRoom({
					id: props.room.id,
					name: value.name.trim(),
					description: value.description.trim(),
					limit: value.limit,
					audio_only: value.audioOnly,
					allow_audience: value.allowAudience,
				});
				await props.onSaved(updated);
				showToast(`已更新房间: ${updated.name}`, { type: "success" });
				props.onClose();
			} catch {}
		},
	}));

	const fields: FormFieldConfig[] = [
		{
			name: "name",
			label: "房间名称",
			type: "text",
			placeholder: "例如：产品评审会",
			required: true,
			validation: roomNameValidation,
		},
		{
			name: "limit",
			label: "人数上限",
			type: "number",
			required: true,
			validation: limitValidation,
		},
		{
			name: "audioOnly",
			label: "仅语音模式",
			type: "switch",
		},
		{
			name: "allowAudience",
			label: "允许听众加入",
			type: "switch",
		},
		{
			name: "description",
			label: "房间说明",
			type: "textarea",
			placeholder: "选填，用于说明当前房间的主题或规则",
			validation: (value: string) =>
				value.trim().length > 120 ? "房间说明不能超过 120 个字符" : undefined,
			className: "min-h-24",
		},
	];

	return (
		<dialog ref={props.ref} class="modal" id="edit_room_modal">
			<div class="modal-box max-w-xl rounded-lg p-0">
				<button
					class="top-2 right-2 absolute border-0 z-10 btn btn-sm btn-circle"
					onClick={props.onClose}
				>
					✕
				</button>
				<div class="border-base-300 border-b px-6 py-5">
					<h3 class="text-lg font-semibold">编辑房间</h3>
					<p class="mt-1 text-sm text-base-content/60">
						修改房间名称、说明、人数上限与接入策略。密码不在本页修改。
					</p>
				</div>
				<div class="grid gap-6 px-6 py-5">
					<Form
						form={form}
						fields={fields}
						formClassName="grid gap-2 md:grid-cols-2 md:gap-x-4"
						submitButtonText="保存修改"
					/>
				</div>
				<form method="dialog" class="modal-backdrop">
					<button onClick={props.onClose}></button>
				</form>
			</div>
		</dialog>
	);
};

export default EditRoomModal;
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd app/web && pnpm exec vitest run src/components/domain/DomainRoomTable.spec.tsx`
Expected: PASS（5 个校验用例全绿）。
Run: `cd app/web && pnpm exec tsc --noEmit`
Expected: 退出码 0。

- [ ] **Step 5: 提交**

```bash
git add app/web/src/components/modal/editRoomModal.tsx app/web/src/components/domain/DomainRoomTable.spec.tsx
git commit -m "feat(web): add edit room modal"
```

---

### Task 5: 新增 DomainRoomTable + spec

**Files:**
- Modify: `app/web/src/components/domain/DomainMemberTable.tsx`
- Create: `app/web/src/components/domain/DomainRoomTable.tsx`
- Modify: `app/web/src/components/domain/DomainRoomTable.spec.tsx`（Task 4 已建，本任务追加用例）

**Interfaces:**
- Consumes: `RoomRecord`/`RoomListResult`（Task 2）、`formatDate`（本任务从 DomainMemberTable 导出）、`getRoomTableStatus` 状态机模式（对齐 `DomainMemberTable`）。
- Produces: `DomainRoomTable`（props 见下）、纯函数 `roomTypeLabel`/`canManageRoomAction`/`getRoomActions`/`getRoomTableStatus`/`isRoomTableBusy`。Task 6 依赖本任务。

- [ ] **Step 1: 导出 formatDate**

修改 `app/web/src/components/domain/DomainMemberTable.tsx` 第 110 行：

```tsx
export function formatDate(value?: string) {
```

（其余不变。）

- [ ] **Step 2: 追加 spec 用例**

在 `app/web/src/components/domain/DomainRoomTable.spec.tsx` 的 import 与 describe 块中追加（保留 Task 4 已写的校验 describe）：

```tsx
import type { RoomRecord } from "@/api/room";
import {
	canManageRoomAction,
	getRoomActions,
	getRoomTableStatus,
	isRoomTableBusy,
	roomTypeLabel,
} from "./DomainRoomTable";

const room: RoomRecord = {
	id: 1,
	uuid: "r-1",
	name: "lobby",
	description: "",
	limit: 12,
	audio_only: true,
	allow_audience: true,
	type: "voice",
	created_by: "creator-1",
	created_at: "2026-08-02T00:00:00Z",
	domain_uuid: "g-1",
};

describe("DomainRoomTable type labels", () => {
	it("labels text rooms and voice rooms", () => {
		expect(roomTypeLabel("text")).toBe("文字");
		expect(roomTypeLabel("voice")).toBe("语音");
		expect(roomTypeLabel(undefined)).toBe("语音");
	});
});

describe("DomainRoomTable action logic", () => {
	it("hides actions when the user lacks manage rights and did not create the room", () => {
		expect(canManageRoomAction(room, "other-1", false)).toBe(false);
		expect(getRoomActions(room, "other-1", false, vi.fn(), vi.fn())).toBeNull();
	});

	it("shows actions to the room creator", () => {
		expect(canManageRoomAction(room, "creator-1", false)).toBe(true);
		expect(
			getRoomActions(room, "creator-1", false, vi.fn(), vi.fn()),
		).not.toBeNull();
	});

	it("shows actions when the user can manage the domain", () => {
		expect(canManageRoomAction(room, "other-1", true)).toBe(true);
	});

	it("builds edit and delete actions that call back with the room", () => {
		const onEdit = vi.fn();
		const onDelete = vi.fn();
		const actions = getRoomActions(room, "creator-1", false, onEdit, onDelete);

		expect(actions).not.toBeNull();
		expect(actions?.map((a) => a.label)).toEqual(["编辑", "删除"]);
		actions?.[0].onClick();
		expect(onEdit).toHaveBeenCalledWith(room);
		actions?.[1].onClick();
		expect(onDelete).toHaveBeenCalledWith(room);
	});
});

describe("DomainRoomTable status logic", () => {
	it("shows loading while no rooms have loaded", () => {
		expect(getRoomTableStatus([], true, null)).toBe("loading");
	});

	it("shows error when loading fails without cached rooms", () => {
		expect(getRoomTableStatus([], false, "network")).toBe("error");
	});

	it("keeps rows visible and reports an error when refresh fails", () => {
		expect(getRoomTableStatus([room], false, "network")).toBe(
			"ready-with-error",
		);
	});

	it("shows empty state when there are no rooms and no error", () => {
		expect(getRoomTableStatus([], false, null)).toBe("empty");
	});

	it("treats initial loading and refresh as busy states", () => {
		expect(isRoomTableBusy(true, false)).toBe(true);
		expect(isRoomTableBusy(false, true)).toBe(true);
		expect(isRoomTableBusy(false, false)).toBe(false);
	});
});
```

需把 spec 顶部 import 行的 `import { describe, expect, it } from "vitest";` 改为 `import { describe, expect, it, vi } from "vitest";`。

- [ ] **Step 3: 运行测试确认失败**

Run: `cd app/web && pnpm exec vitest run src/components/domain/DomainRoomTable.spec.tsx`
Expected: FAIL（`./DomainRoomTable` 不存在）。

- [ ] **Step 4: 实现 DomainRoomTable**

创建 `app/web/src/components/domain/DomainRoomTable.tsx`：

```tsx
import type { Component } from "solid-js";
import { For, Show } from "solid-js";
import Pencil from "lucide-solid/icons/pencil";
import Trash2 from "lucide-solid/icons/trash-2";
import type { RoomRecord } from "@/api/room";
import { formatDate } from "./DomainMemberTable";

export type RoomTableStatus =
	| "loading"
	| "error"
	| "empty"
	| "ready"
	| "ready-with-error";

export function roomTypeLabel(type?: "text" | "voice") {
	return type === "text" ? "文字" : "语音";
}

export interface RoomAction {
	label: string;
	ariaLabel: string;
	onClick: () => void;
}

export function canManageRoomAction(
	room: Pick<RoomRecord, "created_by" | "name">,
	currentUserName: string | undefined,
	canManage: boolean,
) {
	return canManage || (!!currentUserName && room.created_by === currentUserName);
}

export function getRoomActions(
	room: Pick<RoomRecord, "created_by" | "name">,
	currentUserName: string | undefined,
	canManage: boolean,
	onEdit: (room: RoomRecord) => void,
	onDelete: (room: RoomRecord) => void,
): RoomAction[] | null {
	if (!canManageRoomAction(room, currentUserName, canManage)) return null;
	return [
		{
			label: "编辑",
			ariaLabel: `编辑 ${room.name}`,
			onClick: () => onEdit(room as RoomRecord),
		},
		{
			label: "删除",
			ariaLabel: `删除 ${room.name}`,
			onClick: () => onDelete(room as RoomRecord),
		},
	];
}

export function getRoomTableStatus(
	rooms: RoomRecord[],
	loading: boolean,
	error: string | null,
): RoomTableStatus {
	if (rooms.length === 0 && loading) return "loading";
	if (rooms.length === 0 && error) return "error";
	if (rooms.length === 0) return "empty";
	return error ? "ready-with-error" : "ready";
}

export function isRoomTableBusy(loading: boolean, refreshing: boolean) {
	return loading || refreshing;
}

export interface DomainRoomTableProps {
	rooms: RoomRecord[];
	currentUserName?: string;
	canManage?: boolean;
	loading?: boolean;
	refreshing?: boolean;
	error?: string | null;
	page: number;
	totalPages: number;
	onPageChange: (page: number) => void;
	onRefresh?: () => void;
	onEdit: (room: RoomRecord) => void;
	onDelete: (room: RoomRecord) => void;
}

const DomainRoomTable: Component<DomainRoomTableProps> = (props) => {
	const rooms = () => props.rooms ?? [];
	const loading = () => props.loading ?? false;
	const refreshing = () => props.refreshing ?? false;
	const error = () => props.error || null;
	const busy = () => isRoomTableBusy(loading(), refreshing());
	const status = () => getRoomTableStatus(rooms(), loading(), error());

	return (
		<div class="min-w-0">
			<div class="overflow-x-auto">
				<table class="table table-sm" aria-busy={busy()}>
					<thead>
						<tr>
							<th>名称</th>
							<th>类型</th>
							<th>创建者</th>
							<th>人数上限</th>
							<th>创建时间</th>
							<th class="text-right">操作</th>
						</tr>
					</thead>
					<tbody>
						<Show when={status() === "loading"}>
							<tr>
								<td
									colSpan={6}
									class="py-10 text-center text-sm text-base-content/50"
								>
									<span class="loading loading-spinner loading-sm" />
									<span class="ml-2">正在加载房间</span>
								</td>
							</tr>
						</Show>
						<For each={rooms()}>
							{(room) => {
								const actions = () =>
									getRoomActions(
										room,
										props.currentUserName,
										props.canManage ?? false,
										props.onEdit,
										props.onDelete,
									);

								return (
									<tr>
										<td class="min-w-0 max-w-[260px]">
											<div class="truncate font-medium">
												{room.type === "text" ? "# " : ""}
												{room.name}
											</div>
										</td>
										<td>
											<span class="badge badge-ghost badge-sm">
												{roomTypeLabel(room.type)}
											</span>
										</td>
										<td class="text-xs text-base-content/60">
											{room.created_by || "-"}
										</td>
										<td class="text-xs">{room.limit}</td>
										<td class="text-xs text-base-content/60 whitespace-nowrap">
											{formatDate(room.created_at)}
										</td>
										<td class="text-right">
											<Show when={actions()}>
												<div class="flex items-center justify-end gap-1">
													<button
														type="button"
														class="btn btn-outline btn-xs"
														aria-label={actions()?.[0].ariaLabel}
														onClick={() => actions()?.[0].onClick()}
													>
														<Pencil size={14} />
														编辑
													</button>
													<button
														type="button"
														class="btn btn-outline btn-error btn-xs"
														aria-label={actions()?.[1].ariaLabel}
														onClick={() => actions()?.[1].onClick()}
													>
														<Trash2 size={14} />
														删除
													</button>
												</div>
											</Show>
										</td>
									</tr>
								);
							}}
						</For>
						<Show when={status() === "empty"}>
							<tr>
								<td
									colSpan={6}
									class="py-10 text-center text-sm text-base-content/50"
								>
									暂无房间
								</td>
							</tr>
						</Show>
					</tbody>
				</table>
			</div>

			<Show when={status() === "error"}>
				<div
					role="alert"
					class="flex flex-wrap items-center justify-between gap-3 border-t border-base-300 bg-base-200/40 px-4 py-3 text-sm"
				>
					<span class="text-error">房间加载失败：{error()}</span>
					<Show when={props.onRefresh}>
						<button
							type="button"
							class="btn btn-outline btn-sm"
							disabled={busy()}
							onClick={() => props.onRefresh?.()}
						>
							重试
						</button>
					</Show>
				</div>
			</Show>
			<Show when={status() === "ready-with-error"}>
				<div
					role="alert"
					class="flex flex-wrap items-center justify-between gap-3 border-t border-base-300 bg-base-200/40 px-4 py-3 text-sm"
				>
					<span class="text-error">房间刷新失败，已保留现有数据</span>
					<Show when={props.onRefresh}>
						<button
							type="button"
							class="btn btn-outline btn-sm"
							disabled={busy()}
							onClick={() => props.onRefresh?.()}
						>
							刷新
						</button>
					</Show>
				</div>
			</Show>

			<div class="flex items-center justify-between border-t border-base-300 px-4 py-3">
				<button
					type="button"
					class="btn btn-sm"
					disabled={props.page <= 1}
					onClick={() => props.onPageChange(props.page - 1)}
				>
					上一页
				</button>
				<span class="text-sm text-base-content/60">
					{props.page} / {props.totalPages}
				</span>
				<button
					type="button"
					class="btn btn-sm"
					disabled={props.page >= props.totalPages}
					onClick={() => props.onPageChange(props.page + 1)}
				>
					下一页
				</button>
			</div>
		</div>
	);
};

export default DomainRoomTable;
```

注：组件头部刷新按钮由页面 ManageSection actions 提供，组件内无独立刷新按钮，故**不导入** `RefreshCw`（如上 import 块所示）。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd app/web && pnpm exec vitest run src/components/domain/DomainRoomTable.spec.tsx`
Expected: PASS（类型标签 3 + 操作 4 + 状态机 5 + 校验 5 = 17 用例全绿）。
Run: `cd app/web && pnpm exec tsc --noEmit`
Expected: 退出码 0。
Run: `cd app/web && pnpm exec biome check`
Expected: 退出码 0；格式漂移先 `pnpm exec biome format --write src/components/domain/DomainRoomTable.tsx src/components/domain/DomainMemberTable.tsx src/components/domain/DomainRoomTable.spec.tsx` 再复跑。

- [ ] **Step 6: 提交**

```bash
git add app/web/src/components/domain/DomainRoomTable.tsx app/web/src/components/domain/DomainRoomTable.spec.tsx app/web/src/components/domain/DomainMemberTable.tsx
git commit -m "feat(web): add domain room table"
```

---

### Task 6: 域管理页集成「房间管理」区块

**Files:**
- Modify: `app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx`

**Interfaces:**
- Consumes: `listRooms`/`deleteRoom`/`RoomRecord`（Task 2）、`CreateRoomModal` + `domainUUID` prop（Task 3）、`EditRoomModal`（Task 4）、`DomainRoomTable`（Task 5）、`hasAnyPermission`（`@/utils/permissions`）、`untrack`（solid-js）。
- Produces: 页面房间管理区（可管理时显示「新建房间」、表格、分页、编辑/删除弹窗）。PAGE_SIZE=10。

- [ ] **Step 1: 更新 import**

`app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx` 的 import 变更：

1a. solid-js 行改为：

```tsx
import { createEffect, createMemo, createSignal, Show, untrack } from "solid-js";
```

1b. 新增 icon import（放 lucide 分组，按字母序，`Settings2` 之后）：

```tsx
import CirclePlus from "lucide-solid/icons/circle-plus";
```

1c. api 分组新增：

```tsx
import { deleteRoom, listRooms, type RoomRecord } from "@/api/room";
```

1d. 组件 import 新增：

```tsx
import DomainRoomTable from "@/components/domain/DomainRoomTable";
import CreateRoomModal from "@/components/modal/createRoomModal";
import EditRoomModal from "@/components/modal/editRoomModal";
```

1e. permissions import 行改为：

```tsx
import { hasAnyPermission, hasPermission } from "@/utils/permissions";
```

- [ ] **Step 2: 新增房间状态与函数**

在 `const canKick = ...` 之后（第 92 行 `);` 之后、`const [name, setName] = createSignal("");` 之前）插入：

```tsx
	const ROOM_PAGE_SIZE = 10;
	const [rooms, setRooms] = createSignal<RoomRecord[]>([]);
	const [roomTotal, setRoomTotal] = createSignal(0);
	const [roomPage, setRoomPage] = createSignal(1);
	const [roomLoading, setRoomLoading] = createSignal(true);
	const [roomRefreshing, setRoomRefreshing] = createSignal(false);
	const [roomError, setRoomError] = createSignal("");
	const [editingRoom, setEditingRoom] = createSignal<RoomRecord | null>(null);
	const [deletingRoom, setDeletingRoom] = createSignal<RoomRecord | null>(null);
	const [deleting, setDeleting] = createSignal(false);
	const [deleteError, setDeleteError] = createSignal("");
	const totalRoomPages = createMemo(() =>
		Math.max(1, Math.ceil(roomTotal() / ROOM_PAGE_SIZE)),
	);
	const canManageRooms = createMemo(
		() =>
			isOwner() ||
			currentRole() === "admin" ||
			hasAnyPermission("room:update", "room:delete"),
	);
	let createRoomDialogRef!: HTMLDialogElement;
	let editRoomDialogRef!: HTMLDialogElement;
	let deleteDialogRef!: HTMLDialogElement;

	async function fetchRooms(page: number) {
		setRoomError("");
		if (rooms().length > 0) setRoomRefreshing(true);
		else setRoomLoading(true);
		try {
			const result = await listRooms(page, ROOM_PAGE_SIZE, uuid());
			setRooms(result.rooms);
			setRoomTotal(result.total);
			setRoomPage(result.page);
		} catch (error) {
			setRoomError(apiErrorMessage(error));
		} finally {
			setRoomLoading(false);
			setRoomRefreshing(false);
		}
	}

	function resetRooms() {
		setRooms([]);
		setRoomTotal(0);
		setRoomPage(1);
		setRoomError("");
	}

	function openEditRoom(room: RoomRecord) {
		setEditingRoom(room);
		queueMicrotask(() => editRoomDialogRef?.showModal?.());
	}

	function requestDeleteRoom(room: RoomRecord) {
		setDeleteError("");
		setDeletingRoom(room);
		queueMicrotask(() => deleteDialogRef?.showModal?.());
	}

	function closeDeleteModal() {
		deleteDialogRef?.close();
		setDeletingRoom(null);
		setDeleteError("");
	}

	async function handleDeleteRoom() {
		const room = deletingRoom();
		if (!room) return;
		setDeleting(true);
		setDeleteError("");
		try {
			await deleteRoom(room.id);
			closeDeleteModal();
			showToast("房间已删除", { type: "success" });
			void fetchRooms(roomPage());
		} catch (error) {
			const message = apiErrorMessage(error);
			setDeleteError(message);
			showToast(message, { type: "error" });
		} finally {
			setDeleting(false);
		}
	}
```

- [ ] **Step 3: 进入/切换域时加载房间**

现有域名加载 effect（第 110-114 行）替换为：

```tsx
	createEffect(() => {
		const currentUUID = uuid();
		setCurrentDomain(currentUUID);
		void loadMembers(currentUUID).catch(() => {});
		untrack(() => {
			resetRooms();
			void fetchRooms(1);
		});
	});
```

`untrack` 包裹保证该 effect 只订阅 `uuid()`，不因 `rooms()`/`roomPage()` 等内部状态变化自触发循环。

- [ ] **Step 4: 插入「房间管理」ManageSection**

在两列网格的闭合 `</div>`（第 441 行）之后、外层 `</Show>`（`<Show when={domain()}>`）之前插入：

```tsx
					<ManageSection
						title="房间管理"
						description={`${roomTotal()} 个房间`}
						padded={false}
						actions={
							<>
								<button
									type="button"
									class="btn btn-ghost btn-xs"
									disabled={roomLoading() || roomRefreshing()}
									onClick={() => {
										resetRooms();
										void fetchRooms(1);
									}}
								>
									{roomLoading() || roomRefreshing() ? (
										<span class="loading loading-spinner loading-xs" />
									) : null}
									刷新
								</button>
								<Show when={canManageRooms()}>
									<button
										type="button"
										class="btn btn-primary btn-xs"
										onClick={() => createRoomDialogRef?.showModal?.()}
									>
										<CirclePlus size={14} />
										新建房间
									</button>
								</Show>
							</>
						}
					>
						<DomainRoomTable
							rooms={rooms()}
							currentUserName={currentUser()?.name}
							canManage={canManageRooms()}
							loading={roomLoading()}
							refreshing={roomRefreshing()}
							error={roomError()}
							page={roomPage()}
							totalPages={totalRoomPages()}
							onPageChange={(page) => void fetchRooms(page)}
							onRefresh={() => {
								resetRooms();
								void fetchRooms(1);
							}}
							onEdit={openEditRoom}
							onDelete={requestDeleteRoom}
						/>
					</ManageSection>
```

- [ ] **Step 5: 挂载弹窗**

在踢人 ConfirmModal（第 485 行 `/>` 之后、外层 `<div>` 闭合之前）追加：

```tsx
				<CreateRoomModal
					ref={createRoomDialogRef}
					domainUUID={uuid()}
					onClose={() => createRoomDialogRef?.close?.()}
					onCreated={() => {
						resetRooms();
						void fetchRooms(1);
					}}
				/>
				<Show when={editingRoom()}>
					{(room) => (
						<EditRoomModal
							ref={editRoomDialogRef}
							room={room()}
							onClose={() => editRoomDialogRef?.close?.()}
							onSaved={() => {
								void fetchRooms(roomPage());
							}}
						/>
					)}
				</Show>
				<ConfirmModal
					open={!!deletingRoom()}
					title="删除房间"
					message={
						<span>
							确认删除房间{" "}
							{deletingRoom()?.name || "该房间"}？删除后不可恢复。
							<Show when={deleteError()}>
								<span class="mt-2 block text-error">{deleteError()}</span>
							</Show>
						</span>
					}
					confirmText="删除"
					confirmClass="btn btn-error"
					loading={deleting()}
					dialogRef={(el) => {
						deleteDialogRef = el;
					}}
					onClose={closeDeleteModal}
					onConfirm={handleDeleteRoom}
				/>
```

- [ ] **Step 6: 验证**

Run: `cd app/web && pnpm exec tsc --noEmit`
Expected: 退出码 0。
Run: `cd app/web && pnpm exec vitest run`
Expected: 全部 PASS（含既有 DomainMemberTable/DomainIcon/DomainInvitePreview spec 与本功能 spec）。
Run: `cd app/web && pnpm exec biome check`
Expected: 退出码 0；格式漂移先 `pnpm exec biome format --write "src/pages/(app)/manage/domains/\$domainUUID/index.tsx"` 再复跑。
Run: `cd app/server && go test ./internal/handler/ -count=1 && cd app/server && go build ./...`
Expected: 全部 PASS + 编译通过（回归确认后端未受影响）。

- [ ] **Step 7: 提交**

```bash
git add "app/web/src/pages/(app)/manage/domains/$domainUUID/index.tsx"
git commit -m "feat(web): manage rooms from domain page"
```

---

## 手动验证（交付前）

Dev 环境按规格验证：
1. 建两用户：普通成员 vs owner/admin，进入 `/manage/domains/$domainUUID`——普通成员对他人房间无「编辑/删除」，对自建房间有；owner/admin 对全部房间有。
2. 新建房间归属当前域；编辑名称/说明/上限/仅语音/允许听众生效；删除走确认弹窗后列表刷新。
3. 分页：造 >10 房间，翻页正确；空域显示「暂无房间」。
4. 权限不足时后端 403 → 统一错误 toast。

结果以 Markdown 存入 `agent_test_logs/`，命名 `{内容}-{时间}.md`。
