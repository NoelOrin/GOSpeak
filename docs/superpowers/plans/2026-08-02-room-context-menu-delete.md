# 房间右键菜单新增删除房间 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在房间右键菜单中为有权限用户新增“删除房间”入口，并复用现有删除接口完成确认与列表刷新。

**Architecture:** 前端只改房间列表相关文件。新增 `canDeleteRoomItem` 权限 helper，`RoomItem` 通过 `onDeleteRoom` 回调把待删房间交给 `RoomList`，由 `RoomList` 使用 `ConfirmModal` 集中处理确认、请求、错误和刷新。

**Tech Stack:** SolidJS、TypeScript、Vite、Vitest、daisyui、lucide-solid。

> 项目约束：本计划不执行 git commit；按 `AGENTS.md` 要求，提交需用户明确同意。

---

### Task 1: 新增删除权限 helper

**Files:**
- Modify: `app/web/src/components/room/roomListUtils.ts`
- Modify: `app/web/src/components/room/roomListUtils.spec.ts`

- [ ] **Step 1: 写失败的权限判断测试**

将 `app/web/src/components/room/roomListUtils.spec.ts` 顶部 import 改为：

```ts
import { describe, expect, it } from "vitest";
import type { RoomInfo } from "@/socket/types";
import {
	canDeleteRoomItem,
	canEditRoomItem,
	toEditRoomRecord,
} from "./roomListUtils";
```

在该文件 `describe("canEditRoomItem", ...)` 之后新增：

```ts
describe("canDeleteRoomItem", () => {
	it("grants delete access to domain owners", () => {
		expect(
			canDeleteRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-1" },
				"member",
				false,
			),
		).toBe(true);
	});

	it("grants delete access to domain admins", () => {
		expect(
			canDeleteRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-2" },
				"admin",
				false,
			),
		).toBe(true);
	});

	it("grants delete access to users with room:delete", () => {
		expect(canDeleteRoomItem({ uuid: "user-1" }, null, "member", true)).toBe(
			true,
		);
	});

	it("denies ordinary members without room:delete", () => {
		expect(
			canDeleteRoomItem(
				{ uuid: "user-1" },
				{ owner_uuid: "user-2" },
				"member",
				false,
			),
		).toBe(false);
	});
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --dir app/web test src/components/room/roomListUtils.spec.ts`

Expected: FAIL，错误包含 `canDeleteRoomItem is not a function` 或 `canDeleteRoomItem` 未定义。

- [ ] **Step 3: 实现 helper**

在 `app/web/src/components/room/roomListUtils.ts` 的 `canEditRoomItem` 之后新增：

```ts
export function canDeleteRoomItem(
	currentUser: { uuid?: string } | null,
	domain: { owner_uuid?: string } | null,
	memberRole: string | null | undefined,
	hasRoomDeletePermission: boolean,
) {
	return (
		hasRoomDeletePermission ||
		(!!currentUser?.uuid && domain?.owner_uuid === currentUser.uuid) ||
		memberRole === "admin"
	);
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --dir app/web test src/components/room/roomListUtils.spec.ts`

Expected: PASS，`canDeleteRoomItem` 的 4 个用例全部通过。

---

### Task 2: RoomItem 右键菜单新增删除项

**Files:**
- Modify: `app/web/src/components/room/roomList.tsx`

- [ ] **Step 1: 增加 import 和 prop**

在 `import Pencil from "lucide-solid/icons/pencil";` 之后新增：

```ts
import Trash2 from "lucide-solid/icons/trash-2";
```

在 `import { type RoomInfo, socketStore } from "@/stores/socketStore";` 之后新增：

```ts
import { showToast } from "solid-notifications";
```

在 `import { canEditRoomItem, toEditRoomRecord } from "./roomListUtils";` 后改为：

```ts
import {
	canDeleteRoomItem,
	canEditRoomItem,
	toEditRoomRecord,
} from "./roomListUtils";
```

在 `RoomItemPropsType` 接口的 `onEditRoom` 声明后新增：

```ts
	onDeleteRoom: (room: RoomInfo) => void;
```

- [ ] **Step 2: 增加 canDelete 计算**

在 `const canEdit = () => ...;` 之后新增：

```ts
	const canDelete = () =>
		canDeleteRoomItem(
			userStore.user(),
			socketStore.currentDomainUUID()
				? (domainStore.state.domainCache[
						socketStore.currentDomainUUID() ?? ""
					] ?? null)
				: null,
			socketStore.currentDomainUUID()
				? domainStore.state.memberCache[
						socketStore.currentDomainUUID() ?? ""
					]?.find((member) => member.user_uuid === userStore.user()?.uuid)
						?.role_name
				: undefined,
			hasPermission("room:delete"),
		);
```

- [ ] **Step 3: 调整右键菜单打开条件与高度**

将 `openContextMenu` 内的：

```ts
		if (!canEdit() && !isInRoom()) return;
		event.preventDefault();
		const menuWidth = 176;
		const menuHeight = 132;
```

改为：

```ts
		if (!canEdit() && !canDelete() && !isInRoom()) return;
		event.preventDefault();
		const menuWidth = 176;
		const visibleMenuCount = [canEdit(), canDelete(), isInRoom()].filter(
			Boolean,
		).length;
		const menuHeight = visibleMenuCount * 40 + 8;
```

- [ ] **Step 4: 增加删除点击处理**

在 `const handleEdit = () => ...;` 之后新增：

```ts
	const handleDelete = () => {
		closeContextMenu();
		props.onDeleteRoom(props.room);
	};
```

- [ ] **Step 5: 渲染删除菜单项**

在现有“退出房间”按钮的 `</Show>` 之后新增：

```tsx
							<Show when={canDelete()}>
								<button
									type="button"
									role="menuitem"
									class="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm text-error transition-colors hover:bg-error/10 focus-visible:bg-error/10 focus:outline-none"
									onClick={handleDelete}
								>
									<Trash2 size={14} />
									<span>删除房间</span>
								</button>
							</Show>
```

- [ ] **Step 6: 类型检查**

Run: `pnpm --dir app/web exec tsc --noEmit`

Expected: PASS，或仅出现与后续 `RoomList` 尚未传 `onDeleteRoom` 相关的类型错误。

---

### Task 3: RoomList 集中删除确认流程

**Files:**
- Modify: `app/web/src/components/room/roomList.tsx`

- [ ] **Step 1: 增加错误提取函数**

在 `// 子项` 注释之前新增：

```ts
function apiErrorMessage(error: unknown): string {
	const response = (error as { response?: { data?: { msg?: string } } })?.response
		?.data?.msg;
	if (response) return response;
	if (error instanceof Error) return error.message;
	return "删除房间失败";
}
```

- [ ] **Step 2: 增加 import**

在 `import { type RoomInfo, socketStore } from "@/stores/socketStore";` 之后新增：

```ts
import { deleteRoom } from "@/api/room";
import ConfirmModal from "@/components/common/ConfirmModal";
```

- [ ] **Step 3: 增加删除状态**

在 `const [editingRoom, setEditingRoom] = createSignal<RoomInfo | null>(null);` 之后新增：

```ts
	const [deletingRoom, setDeletingRoom] = createSignal<RoomInfo | null>(null);
	const [deleting, setDeleting] = createSignal(false);
	const [deleteError, setDeleteError] = createSignal("");
	let deleteDialogRef!: HTMLDialogElement;
```

- [ ] **Step 4: 增加删除处理方法**

在 `const openEditRoom = (room: RoomInfo) => ...;` 之后新增：

```ts
	const openDeleteRoom = (room: RoomInfo) => {
		setDeleteError("");
		setDeletingRoom(room);
		queueMicrotask(() => deleteDialogRef?.showModal?.());
	};

	const closeDeleteModal = () => {
		deleteDialogRef?.close();
		setDeletingRoom(null);
		setDeleteError("");
	};

	const handleDeleteRoom = async () => {
		const room = deletingRoom();
		if (!room) return;
		setDeleting(true);
		setDeleteError("");
		try {
			await deleteRoom(room.id);
			closeDeleteModal();
			showToast("房间已删除", { type: "success" });
			socketStore.listRooms();
		} catch (error) {
			const message = apiErrorMessage(error);
			setDeleteError(message);
			showToast(message, { type: "error" });
		} finally {
			setDeleting(false);
		}
	};
```

- [ ] **Step 5: 给 RoomItem 传入回调**

在 `<RoomItem` 的 `onEditRoom={openEditRoom}` 后新增：

```tsx
											onDeleteRoom={openDeleteRoom}
```

- [ ] **Step 6: 渲染确认弹窗**

在现有 `EditRoomModal` 的 `</Show>` 之后新增：

```tsx
			<Show when={deletingRoom()}>
				{(room) => (
					<ConfirmModal
						open
						title="删除房间"
						message={
							<span>
								确认删除房间 {room().name}？删除后不可恢复。
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
				)}
			</Show>
```

- [ ] **Step 7: 运行类型检查**

Run: `pnpm --dir app/web exec tsc --noEmit`

Expected: PASS。

---

### Task 4: 验证与收尾

**Files:**
- No new file changes.

- [ ] **Step 1: 运行权限单测**

Run: `pnpm --dir app/web test src/components/room/roomListUtils.spec.ts`

Expected: PASS。

- [ ] **Step 2: 运行 Biome check**

Run: `pnpm --dir app/web check`

Expected: PASS，无 lint 或格式错误。如报格式问题，执行 `pnpm --dir app/web format` 后再次 check。

- [ ] **Step 3: 运行前端构建**

Run: `pnpm --dir app/web build`

Expected: PASS，Vite 构建和 TypeScript 检查均通过。

- [ ] **Step 4: 启动 dev server 供人工验证**

Run: `pnpm --filter @gospeak/web dev`

Expected: dev server 正常启动，提供本地 URL。人工验证：
- 有 `room:delete` 权限、域所有者或域 `admin` 时右键显示“删除房间”。
- 普通成员右键不显示“删除房间”。
- 点击“删除房间”后出现确认弹窗，确认后删除成功并刷新列表。
