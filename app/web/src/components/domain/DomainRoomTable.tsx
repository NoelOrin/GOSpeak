import { Pencil, Trash2 } from "lucide-react";
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

export function canManageRoomAction(
	room: Pick<RoomRecord, "created_by" | "name">,
	currentUserName: string | undefined,
	canManage: boolean,
) {
	return (
		canManage || (!!currentUserName && room.created_by === currentUserName)
	);
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

export default function DomainRoomTable(props: DomainRoomTableProps) {
	const rooms = props.rooms ?? [];
	const loading = props.loading ?? false;
	const refreshing = props.refreshing ?? false;
	const error = props.error || null;
	const busy = isRoomTableBusy(loading, refreshing);
	const status = getRoomTableStatus(rooms, loading, error);

	const btnSm =
		"border-border hover:bg-muted inline-flex h-8 cursor-pointer items-center gap-1 rounded-sm border px-2 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50";

	return (
		<div className="min-w-0">
			<div className="overflow-x-auto">
				<table className="w-full text-sm" aria-busy={busy}>
					<thead>
						<tr className="border-border border-b">
							<th className="px-3 py-2 text-left">名称</th>
							<th className="px-3 py-2 text-left">类型</th>
							<th className="px-3 py-2 text-left">创建者</th>
							<th className="px-3 py-2 text-left">人数上限</th>
							<th className="px-3 py-2 text-left">创建时间</th>
							<th className="px-3 py-2 text-right">操作</th>
						</tr>
					</thead>
					<tbody>
						{status === "loading" && (
							<tr>
								<td
									colSpan={6}
									className="text-faint-foreground py-10 text-center text-sm"
								>
									<span className="border-primary border-t-primary mr-2 inline-block size-4 animate-spin rounded-full border-2" />
									正在加载房间
								</td>
							</tr>
						)}
						{rooms.map((room) => {
							const showActions = canManageRoomAction(
								room,
								props.currentUserName,
								props.canManage ?? false,
							);
							return (
								<tr key={room.id} className="border-border/60 border-b">
									<td className="min-w-0 max-w-[260px] px-3 py-2">
										<div className="truncate font-medium">
											{room.type === "text" ? "# " : ""}
											{room.name}
										</div>
									</td>
									<td className="px-3 py-2">
										<span className="bg-muted text-foreground inline-flex items-center rounded-full px-2 py-0.5 text-xs">
											{roomTypeLabel(room.type)}
										</span>
									</td>
									<td className="text-muted-foreground px-3 py-2 text-xs">
										{room.created_by || "-"}
									</td>
									<td className="px-3 py-2 text-xs">{room.limit}</td>
									<td className="text-muted-foreground px-3 py-2 text-xs whitespace-nowrap">
										{formatDate(room.created_at)}
									</td>
									<td className="px-3 py-2 text-right">
										{showActions && (
											<div className="flex items-center justify-end gap-1">
												<button
													type="button"
													className={btnSm}
													aria-label={`编辑 ${room.name}`}
													onClick={() => props.onEdit(room)}
												>
													<Pencil size={14} />
													编辑
												</button>
												<button
													type="button"
													className="border-destructive/50 text-destructive hover:bg-destructive/10 inline-flex h-8 cursor-pointer items-center gap-1 rounded-sm border px-2 text-xs transition-colors"
													aria-label={`删除 ${room.name}`}
													onClick={() => props.onDelete(room)}
												>
													<Trash2 size={14} />
													删除
												</button>
											</div>
										)}
									</td>
								</tr>
							);
						})}
						{status === "empty" && (
							<tr>
								<td
									colSpan={6}
									className="text-faint-foreground py-10 text-center text-sm"
								>
									暂无房间
								</td>
							</tr>
						)}
					</tbody>
				</table>
			</div>

			{status === "error" && (
				<div
					role="alert"
					className="border-border bg-muted/40 flex flex-wrap items-center justify-between gap-3 border-t px-4 py-3 text-sm"
				>
					<span className="text-destructive">房间加载失败：{error}</span>
					{props.onRefresh && (
						<button
							type="button"
							className={btnSm}
							disabled={busy}
							onClick={() => props.onRefresh?.()}
						>
							重试
						</button>
					)}
				</div>
			)}
			{status === "ready-with-error" && (
				<div
					role="alert"
					className="border-border bg-muted/40 flex flex-wrap items-center justify-between gap-3 border-t px-4 py-3 text-sm"
				>
					<span className="text-destructive">房间刷新失败，已保留现有数据</span>
					{props.onRefresh && (
						<button
							type="button"
							className={btnSm}
							disabled={busy}
							onClick={() => props.onRefresh?.()}
						>
							刷新
						</button>
					)}
				</div>
			)}

			<div className="border-border flex items-center justify-between border-t px-4 py-3">
				<button
					type="button"
					className={btnSm}
					disabled={props.page <= 1}
					onClick={() => props.onPageChange(props.page - 1)}
				>
					上一页
				</button>
				<span className="text-muted-foreground text-sm">
					{props.page} / {props.totalPages}
				</span>
				<button
					type="button"
					className={btnSm}
					disabled={props.page >= props.totalPages}
					onClick={() => props.onPageChange(props.page + 1)}
				>
					下一页
				</button>
			</div>
		</div>
	);
}
