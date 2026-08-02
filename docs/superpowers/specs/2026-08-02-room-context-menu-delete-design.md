# 房间右键菜单新增删除房间

## 背景

房间列表的右键菜单目前只提供“编辑房间”和“退出房间”。管理员无法从房间列表直接删除房间，只能进入域管理页面操作。本次为右键菜单新增“删除房间”入口，仅对有权删除的用户显示。

## 目标

- 在房间右键菜单中新增“删除房间”选项。
- 显示条件与现有“编辑房间”逻辑保持一致：拥有 `room:delete` 权限，或为当前域所有者，或为当前域 `admin`。
- 删除前展示确认弹窗，避免误删。
- 删除成功后刷新房间列表。

## 范围

- 仅修改前端房间列表相关文件。
- 复用现有 `POST /api/v1/room/delete` 接口和 `room:delete` 权限码。
- 不修改后端服务、信令层、SFU 清理逻辑。
- 不修改管理页面的现有删除功能。

## 决策

- 删除入口显示规则使用新 helper `canDeleteRoomItem`，规则与 `canEditRoomItem` 对齐。
- 删除范围仅限数据库记录。若房间当前仍有成员在线，信令层活跃房间会保留到成员清空，这是已确认的可接受行为。
- 删除成功不主动退出当前房间、不清理 `currentRoom`、不清理 `selectedRoomInfo`。
- 删除失败保留确认弹窗并展示错误信息。

## 组件与数据流

### RoomItem

- 新增 `canDelete()` 计算，返回 `canDeleteRoomItem(...)` 结果。
- 新增 `onDeleteRoom(room: RoomInfo)` prop。
- `openContextMenu` 的可打开条件改为 `canEdit() || canDelete() || isInRoom()`。
- 菜单高度改为按当前可见菜单项数量计算，避免三项菜单定位偏移或被截断。
- 菜单新增“删除房间”项，使用 `Trash2` 图标和错误色样式。
- 点击“删除房间”后先关闭右键菜单，再调用 `onDeleteRoom(props.room)`。

### RoomList

- 新增 `deletingRoom`、`deleting`、`deleteError` 信号。
- 新增 `ConfirmModal`，复用现有通用确认组件。
- `onDeleteRoom` 打开确认弹窗并记录待删除房间。
- 确认后调用 `deleteRoom(room.id)`。
- 成功后关闭弹窗、显示 `房间已删除` toast、调用 `socketStore.listRooms()`。
- 失败后保留弹窗并显示错误信息。

### 权限 helper

在 `roomListUtils.ts` 新增：

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

## 错误处理

- 删除请求失败时不清空 `deletingRoom`，确认弹窗保持打开。
- 错误信息使用前端现有 `apiErrorMessage` 风格从 Axios 响应中提取。
- 删除成功后不执行本地乐观移除，以服务端列表刷新结果为准。

## 测试

- 为 `canDeleteRoomItem` 增加单测：
  - 域所有者可删除。
  - 域 `admin` 可删除。
  - 有 `room:delete` 权限可删除。
  - 普通成员无权限时不可删除。
- 运行 `roomListUtils.spec.ts`。
- 运行前端 build 或类型检查确认无类型错误。

## 涉及文件

- `app/web/src/components/room/roomListUtils.ts`
- `app/web/src/components/room/roomListUtils.spec.ts`
- `app/web/src/components/room/roomList.tsx`

## 不在范围

- 后端删除接口变更。
- 信令层删除房间广播事件。
- SFU 房间清理。
- 当前在线成员被删除房间后的自动踢出。
