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
	return (
		canManage || (!!currentUserName && room.created_by === currentUserName)
	);
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
