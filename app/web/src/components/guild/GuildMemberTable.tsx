import type { Component } from "solid-js";
import { For, Show } from "solid-js";
import RefreshCw from "lucide-solid/icons/refresh-cw";
import UserX from "lucide-solid/icons/user-x";
import type { GuildMember } from "@/api/guild";

export interface GuildMemberTableProps {
	members: GuildMember[];
	ownerUUID?: string;
	currentUserUUID?: string;
	canKick?: boolean;
	kickDisabled?: boolean;
	loading?: boolean;
	refreshing?: boolean;
	error?: string | null;
	onRefresh?: () => void;
	onKick: (userUUID: string) => void;
}

export interface KickAction {
	label: string;
	ariaLabel: string;
	disabled: boolean;
	onClick: () => void;
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

export function memberDisplayName(member: GuildMember) {
	return member.nickname || member.user_uuid;
}

export function canKickMember(
	member: Pick<GuildMember, "user_uuid">,
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

export function getKickAction(
	member: Pick<GuildMember, "user_uuid">,
	ownerUUID: string | undefined,
	currentUserUUID: string | undefined,
	canKick: boolean,
	onKick: (userUUID: string) => void,
	options: { disabled?: boolean } = {},
): KickAction | null {
	if (!canKickMember(member, ownerUUID, currentUserUUID, canKick)) {
		return null;
	}

	return {
		label: "踢出",
		ariaLabel: `踢出 ${member.user_uuid}`,
		disabled: !!options.disabled,
		onClick: () => onKick(member.user_uuid),
	};
}

export function getMemberTableStatus(
	members: GuildMember[],
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

export async function executeKickMember(
	guildUUID: string,
	userUUID: string,
	kick: (guildUUID: string, userUUID: string) => Promise<void>,
	refreshMembers: (guildUUID: string) => Promise<unknown>,
) {
	await kick(guildUUID, userUUID);
	await refreshMembers(guildUUID);
}

function formatDate(value?: string) {
	if (!value) return "-";
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return value;
	return date.toLocaleString();
}

const GuildMemberTable: Component<GuildMemberTableProps> = (props) => {
	const members = () => props.members ?? [];
	const loading = () => props.loading ?? false;
	const refreshing = () => props.refreshing ?? false;
	const error = () => props.error || null;
	const busy = () => isMemberTableBusy(loading(), refreshing());
	const status = () => getMemberTableStatus(members(), loading(), error());

	return (
		<div class="min-w-0">
			<div class="flex flex-wrap items-center justify-between gap-3 border-b border-base-300 px-4 py-3">
				<div class="flex items-center gap-2">
					<h2 class="font-semibold">成员管理</h2>
					<span class="badge badge-ghost badge-sm">{members().length}</span>
				</div>
				<div class="flex items-center gap-2">
					<Show when={refreshing() && members().length > 0}>
						<span class="text-xs text-base-content/60">刷新中</span>
					</Show>
					<button
						type="button"
						class="btn btn-ghost btn-xs"
						disabled={busy()}
						onClick={() => props.onRefresh?.()}
					>
						<Show when={busy()} fallback={<RefreshCw size={14} />}>
							<span class="loading loading-spinner loading-xs" />
						</Show>
						刷新
					</button>
				</div>
			</div>

			<div class="overflow-x-auto">
				<table class="table table-sm" aria-busy={busy()}>
					<thead>
						<tr>
							<th>用户</th>
							<th>角色</th>
							<th>加入时间</th>
							<th class="text-right">操作</th>
						</tr>
					</thead>
					<tbody>
						<Show when={status() === "loading"}>
							<tr>
								<td
									colSpan={4}
									class="py-10 text-center text-sm text-base-content/50"
								>
									<span class="loading loading-spinner loading-sm" />
									<span class="ml-2">正在加载成员</span>
								</td>
							</tr>
						</Show>
						<For each={members()}>
							{(member) => {
								const action = () =>
									getKickAction(
										member,
										props.ownerUUID,
										props.currentUserUUID,
										props.canKick ?? false,
										props.onKick,
										{ disabled: props.kickDisabled },
									);

								return (
									<tr>
										<td class="min-w-0 max-w-[260px]">
											<div class="min-w-0">
												<div class="truncate font-medium">
													{memberDisplayName(member)}
												</div>
												<div class="truncate text-xs text-base-content/50">
													{member.user_uuid}
												</div>
											</div>
										</td>
										<td>
											<span class="badge badge-ghost badge-sm">
												{roleLabel(member.role_name)}
											</span>
										</td>
										<td class="text-xs text-base-content/60 whitespace-nowrap">
											{formatDate(member.joined_at)}
										</td>
										<td class="text-right">
											<Show when={action()}>
												<button
													type="button"
													class="btn btn-outline btn-error btn-xs"
													aria-label={action()?.ariaLabel}
													disabled={action()?.disabled}
													onClick={() => action()?.onClick()}
												>
													<UserX size={14} />
													{action()?.label}
												</button>
											</Show>
										</td>
									</tr>
								);
							}}
						</For>
						<Show when={status() === "empty"}>
							<tr>
								<td
									colSpan={4}
									class="py-10 text-center text-sm text-base-content/50"
								>
									暂无成员
								</td>
							</tr>
						</Show>
					</tbody>
				</table>

				<Show when={status() === "error"}>
					<div
						role="alert"
						class="flex flex-wrap items-center justify-between gap-3 border-t border-base-300 bg-base-200/40 px-4 py-3 text-sm"
					>
						<span class="text-error">成员加载失败：{error()}</span>
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
						<span class="text-error">成员刷新失败，已保留现有数据</span>
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
			</div>
		</div>
	);
};

export default GuildMemberTable;
