import Clock from "lucide-solid/icons/clock";
import Gavel from "lucide-solid/icons/gavel";
import InfinityIcon from "lucide-solid/icons/infinity";
import Search from "lucide-solid/icons/search";
import Trash from "lucide-solid/icons/trash";
import UserCheck from "lucide-solid/icons/user-check";
import { For, Show } from "solid-js";
import type { MuteRecord } from "@/api/mute";
import {
	ManageSection,
	ManageTag,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";

export interface UserRow {
	id: number;
	name: string;
	display_name?: string;
	role: string;
}

export interface UsersTableProps {
	loading: boolean;
	users: UserRow[];
	total: number;
	searchInput: string;
	onSearchInput: (value: string) => void;
	onSearch: () => void;
	page: number;
	totalPages: number;
	onPageChange: (page: number) => void;
	roles: string[];
	muteMap: Map<number, MuteRecord>;
	canManageMute: boolean;
	canUpdateUser: boolean;
	canDeleteUser: boolean;
	selfId?: number;
	cancellingId: number | null;
	formatRemaining: (expiresAt: string | null) => string;
	onRoleChange: (userId: number, role: string) => void;
	onStartMute: (userId: number) => void;
	onCancelMute: (userId: number) => void;
	onDelete: (userId: number) => void;
}

function roleLabel(role: string) {
	if (role === "admin") return "管理员";
	if (role === "ban") return "封禁";
	return "用户";
}

export default function UsersTable(props: UsersTableProps) {
	return (
		<ManageSection
			title="用户列表"
			description={`共 ${props.total} 人`}
			padded={false}
		>
			<div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
				<form
					class="flex min-w-72 flex-1 items-center gap-2"
					onSubmit={(e) => {
						e.preventDefault();
						props.onSearch();
					}}
				>
					<label class="input input-bordered input-sm flex min-w-0 flex-1 items-center gap-2">
						<Search size={14} class="shrink-0 text-base-content/40" />
						<input
							type="search"
							class="min-w-0 grow bg-transparent outline-none"
							placeholder="搜索用户名、显示名、邮箱"
							value={props.searchInput}
							onInput={(e) => props.onSearchInput(e.currentTarget.value)}
						/>
					</label>
					<button type="submit" class="btn btn-sm">
						搜索
					</button>
				</form>
				<span class="text-xs text-base-content/50">
					第 {props.page} / {props.totalPages} 页
				</span>
			</div>
			<div class="overflow-x-auto">
				<Show
					when={!props.loading}
					fallback={
						<div class="loading loading-spinner loading-sm mx-auto block py-10" />
					}
				>
					<Show
						when={props.users.length > 0}
						fallback={
							<div class="m-4 rounded-xl border border-dashed border-base-300 bg-base-200/20 py-10 text-center text-sm text-base-content/55">
								暂无用户
							</div>
						}
					>
						<table class="table table-sm">
							<thead>
								<tr class={manageTableHeadClass}>
									<th class="w-16">ID</th>
									<th>用户名</th>
									<th>显示名</th>
									<th class="w-24">角色</th>
									<Show when={props.canManageMute}>
										<th class="w-28">禁言</th>
									</Show>
									<th class="w-48">操作</th>
								</tr>
							</thead>
							<tbody>
								<For each={props.users}>
									{(user) => {
										const mute = () => props.muteMap.get(user.id);
										const muted = () => !!mute();
										const isSelf = () => props.selfId === user.id;
										return (
											<tr
												class={manageTableRowClass}
												classList={{ "bg-base-200/25": muted() }}
											>
												<td class="font-mono text-xs text-base-content/60">
													{user.id}
												</td>
												<td>
													<div class="font-medium text-base-content">
														{user.name}
													</div>
												</td>
												<td class="text-base-content/70">
													{user.display_name || "—"}
												</td>
												<td>
													<ManageTag>{roleLabel(user.role)}</ManageTag>
												</td>
												<Show when={props.canManageMute}>
													<td>
														<Show
															when={muted()}
															fallback={<ManageTag>正常</ManageTag>}
														>
															<ManageTag>
																<span class="inline-flex items-center gap-1">
																	{(mute()?.permanent ?? false) ? (
																		<InfinityIcon size={11} />
																	) : (
																		<Clock size={11} />
																	)}
																	{(mute()?.permanent ?? false)
																		? "永久"
																		: props.formatRemaining(
																				mute()?.expires_at ?? null,
																			)}
																</span>
															</ManageTag>
														</Show>
													</td>
												</Show>
												<td>
													<div class="flex items-center gap-1.5">
														<Show when={props.canUpdateUser}>
															<select
																class="select select-bordered select-xs w-24 bg-base-100"
																value={user.role}
																disabled={isSelf()}
																onChange={(e) =>
																	props.onRoleChange(
																		user.id,
																		e.currentTarget.value,
																	)
																}
															>
																<For each={props.roles}>
																	{(r) => (
																		<option value={r}>{roleLabel(r)}</option>
																	)}
																</For>
															</select>
														</Show>
														<Show when={props.canManageMute}>
															<Show
																when={muted()}
																fallback={
																	<button
																		type="button"
																		class="btn btn-ghost btn-xs h-11 min-h-11 w-11 min-w-11 text-base-content/70"
																		title="禁言"
																		onClick={() => props.onStartMute(user.id)}
																	>
																		<Gavel size={14} />
																	</button>
																}
															>
																<button
																	type="button"
																	class="btn btn-ghost btn-xs h-11 min-h-11 w-11 min-w-11 text-base-content/70"
																	title="解除禁言"
																	disabled={props.cancellingId === user.id}
																	onClick={() => props.onCancelMute(user.id)}
																>
																	<UserCheck size={14} />
																</button>
															</Show>
														</Show>
														<Show when={props.canDeleteUser && !isSelf()}>
															<button
																type="button"
																class="btn btn-ghost btn-xs h-11 min-h-11 w-11 min-w-11 text-base-content/70"
																title="删除用户"
																onClick={() => props.onDelete(user.id)}
															>
																<Trash size={14} />
															</button>
														</Show>
													</div>
												</td>
											</tr>
										);
									}}
								</For>
							</tbody>
						</table>
					</Show>
				</Show>
			</div>
			<div class="flex items-center justify-between gap-3 border-t border-base-300/70 px-4 py-3">
				<span class="text-xs text-base-content/50">共 {props.total} 人</span>
				<div class="flex items-center gap-1.5">
					<button
						type="button"
						class="btn btn-ghost btn-xs"
						disabled={props.page <= 1}
						onClick={() => props.onPageChange(Math.max(1, props.page - 1))}
					>
						上一页
					</button>
					<button
						type="button"
						class="btn btn-ghost btn-xs"
						disabled={props.page >= props.totalPages}
						onClick={() => props.onPageChange(props.page + 1)}
					>
						下一页
					</button>
				</div>
			</div>
		</ManageSection>
	);
}
