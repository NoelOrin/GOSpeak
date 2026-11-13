// biome-ignore-all lint/suspicious/noArrayIndexKey: 骨架占位行数量固定，无业务主键
import {
	Skeleton,
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@gospeak/components";
import { RefreshCw, UserX } from "lucide-react";
import type { DomainMember } from "@/api/domain";

export interface DomainMemberTableProps {
	members: DomainMember[];
	ownerUUID?: string;
	currentUserUUID?: string;
	canKick?: boolean;
	kickDisabled?: boolean;
	loading?: boolean;
	refreshing?: boolean;
	error?: string | null;
	roles?: string[];
	canChangeRole?: boolean;
	onChangeRole?: (userUUID: string, roleName: string) => void;
	onRefresh?: () => void;
	onKick: (userUUID: string) => void;
}

export type MemberTableStatus =
	| "loading"
	| "error"
	| "empty"
	| "ready"
	| "ready-with-error";

export function roleLabel(role: string) {
	switch (role) {
		case "owner":
			return "拥有者";
		case "admin":
			return "管理员";
		case "guest":
			return "访客";
		default:
			return "成员";
	}
}

export function canChangeMemberRole(
	member: Pick<DomainMember, "user_uuid" | "role_name">,
	ownerUUID: string | undefined,
	currentUserUUID: string | undefined,
	canChangeRole: boolean,
	roles?: string[],
): boolean {
	return (
		canChangeRole &&
		!!roles?.length &&
		member.user_uuid !== ownerUUID &&
		member.user_uuid !== currentUserUUID &&
		member.role_name !== "owner"
	);
}

export function memberDisplayName(member: DomainMember) {
	return member.nickname || member.name || member.user_uuid;
}

export function canKickMember(
	member: Pick<DomainMember, "user_uuid">,
	ownerUUID: string | undefined,
	currentUserUUID: string | undefined,
	canKick: boolean,
) {
	return (
		canKick &&
		!!ownerUUID &&
		member.user_uuid !== ownerUUID &&
		member.user_uuid !== currentUserUUID
	);
}

export function getMemberTableStatus(
	members: DomainMember[],
	loading: boolean,
	error: string | null,
): MemberTableStatus {
	if (members.length === 0 && loading) return "loading";
	if (members.length === 0 && error) return "error";
	if (members.length === 0) return "empty";
	return error ? "ready-with-error" : "ready";
}

export function isMemberTableBusy(loading: boolean, refreshing: boolean) {
	return loading || refreshing;
}

export function formatDate(value?: string) {
	if (!value) return "-";
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return date.toLocaleString();
}

const btnGhostXs =
	"hover:bg-muted inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm px-2 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50";

export default function DomainMemberTable(props: DomainMemberTableProps) {
	const members = props.members ?? [];
	const loading = props.loading ?? false;
	const refreshing = props.refreshing ?? false;
	const error = props.error || null;
	const busy = isMemberTableBusy(loading, refreshing);
	const status = getMemberTableStatus(members, loading, error);

	return (
		<div className="min-w-0">
			<div className="border-border flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
				<div className="flex items-center gap-2">
					<h2 className="font-semibold">成员管理</h2>
					<span className="bg-muted text-muted-foreground inline-flex items-center rounded-full px-2 py-0.5 text-xs">
						{members.length}
					</span>
				</div>
				<div className="flex items-center gap-2">
					{refreshing && members.length > 0 && (
						<span className="text-muted-foreground text-xs">刷新中</span>
					)}
					<button
						type="button"
						className={btnGhostXs}
						disabled={busy}
						onClick={() => props.onRefresh?.()}
					>
						{busy ? (
							<span className="border-primary border-t-primary size-3 animate-spin rounded-full border-2" />
						) : (
							<RefreshCw size={14} />
						)}
						刷新
					</button>
				</div>
			</div>

			<div className="overflow-x-auto">
				<table className="w-full text-sm" aria-busy={busy}>
					<thead>
						<tr className="border-border border-b">
							<th className="px-3 py-2 text-left">用户</th>
							<th className="px-3 py-2 text-left">角色</th>
							<th className="px-3 py-2 text-left">加入时间</th>
							<th className="px-3 py-2 text-right">操作</th>
						</tr>
					</thead>
					<tbody>
						{status === "loading" &&
							Array.from({ length: 5 }).map((_, i) => (
								<tr key={`sk-${i}`} className="border-border/60 border-b">
									<td className="px-3 py-2.5">
										<Skeleton className="h-4 w-24" />
									</td>
									<td className="px-3 py-2.5">
										<Skeleton className="h-4 w-16" />
									</td>
									<td className="px-3 py-2.5">
										<Skeleton className="h-4 w-28" />
									</td>
									<td className="px-3 py-2.5">
										<Skeleton className="ml-auto h-4 w-16" />
									</td>
								</tr>
							))}
						{members.map((member) => {
							const canKick = canKickMember(
								member,
								props.ownerUUID,
								props.currentUserUUID,
								props.canKick ?? false,
							);
							const canRole = canChangeMemberRole(
								member,
								props.ownerUUID,
								props.currentUserUUID,
								props.canChangeRole ?? false,
								props.roles,
							);
							return (
								<tr
									key={member.user_uuid}
									className="border-border/60 border-b"
								>
									<td className="min-w-0 max-w-[260px] px-3 py-2">
										<div className="min-w-0">
											<div className="truncate font-medium">
												{memberDisplayName(member)}
											</div>
											<div className="text-faint-foreground truncate text-xs">
												{member.name ? `@${member.name}` : member.user_uuid}
											</div>
										</div>
									</td>
									<td className="px-3 py-2">
										{canRole ? (
											<Select
												value={member.role_name ?? ""}
												onValueChange={(next) =>
													props.onChangeRole?.(member.user_uuid, next)
												}
											>
												<SelectTrigger
													className="h-8 w-auto gap-1 px-2 text-xs"
													aria-label={`角色 ${memberDisplayName(member)}`}
												>
													<SelectValue placeholder="未分配" />
												</SelectTrigger>
												<SelectContent>
													{(props.roles ?? []).map((role) => (
														<SelectItem key={role} value={role}>
															{role}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
										) : (
											<span className="bg-muted text-muted-foreground inline-flex items-center rounded-full px-2 py-0.5 text-xs">
												{roleLabel(member.role_name)}
											</span>
										)}
									</td>
									<td className="text-muted-foreground px-3 py-2 text-xs whitespace-nowrap">
										{formatDate(member.joined_at)}
									</td>
									<td className="px-3 py-2 text-right">
										{canKick && (
											<button
												type="button"
												className="border-destructive/50 text-destructive hover:bg-destructive/10 inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm border px-2 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50"
												aria-label={`踢出 ${member.user_uuid}`}
												disabled={props.kickDisabled}
												onClick={() => props.onKick(member.user_uuid)}
											>
												<UserX size={14} />
												踢出
											</button>
										)}
									</td>
								</tr>
							);
						})}
						{status === "empty" && (
							<tr>
								<td
									colSpan={4}
									className="text-faint-foreground py-10 text-center text-sm"
								>
									暂无成员
								</td>
							</tr>
						)}
					</tbody>
				</table>

				{status === "error" && (
					<div
						role="alert"
						className="border-border bg-muted/40 flex flex-wrap items-center justify-between gap-3 border-t px-4 py-3 text-sm"
					>
						<span className="text-destructive">成员加载失败：{error}</span>
						{props.onRefresh && (
							<button
								type="button"
								className="border-border hover:bg-muted inline-flex h-8 cursor-pointer items-center rounded-sm border px-3 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50"
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
						<span className="text-destructive">
							成员刷新失败，已保留现有数据
						</span>
						{props.onRefresh && (
							<button
								type="button"
								className="border-border hover:bg-muted inline-flex h-8 cursor-pointer items-center rounded-sm border px-3 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50"
								disabled={busy}
								onClick={() => props.onRefresh?.()}
							>
								刷新
							</button>
						)}
					</div>
				)}
			</div>
		</div>
	);
}
