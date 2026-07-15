import Clock from "lucide-solid/icons/clock";
import Gavel from "lucide-solid/icons/gavel";
import InfinityIcon from "lucide-solid/icons/infinity";
import Trash from "lucide-solid/icons/trash";
import UserCheck from "lucide-solid/icons/user-check";
import Users from "lucide-solid/icons/users";
import { For, Show } from "solid-js";
import type { MuteRecord } from "@/api/mute";

export interface UserRow {
	id: number;
	name: string;
	display_name?: string;
	role: string;
}

export interface UsersTableProps {
	loading: boolean;
	users: UserRow[];
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

export default function UsersTable(props: UsersTableProps) {
	return (
		<>
			<div class="flex items-center justify-between gap-3">
				<div class="flex items-center gap-2">
					<Users size={18} />
					<h3 class="font-bold text-lg">用户列表</h3>
					<span class="text-base-content/50 text-xs">
						({props.users.length} 人)
					</span>
				</div>
			</div>

			<div class="overflow-x-auto">
				<Show
					when={!props.loading}
					fallback={
						<div class="loading loading-spinner loading-sm py-8 mx-auto block" />
					}
				>
					<Show
						when={props.users.length > 0}
						fallback={
							<div class="text-base-content/50 py-8 text-center text-sm">
								暂无用户
							</div>
						}
					>
						<table class="table table-zebra table-sm">
							<thead>
								<tr>
									<th>ID</th>
									<th>用户名</th>
									<th>显示名</th>
									<th>角色</th>
									<Show when={props.canManageMute}>
										<th>禁言</th>
									</Show>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								<For each={props.users}>
									{(user) => {
										const mute = () => props.muteMap.get(user.id);
										const muted = () => !!mute();
										const isSelf = () => props.selfId === user.id;
										return (
											<tr classList={{ "bg-warning/5": muted() }}>
												<td class="font-mono text-xs">{user.id}</td>
												<td class="font-medium">{user.name}</td>
												<td>{user.display_name || "—"}</td>
												<td>
													<span
														class="badge badge-xs"
														classList={{
															"badge-primary": user.role === "admin",
															"badge-ghost": user.role !== "admin",
														}}
													>
														{user.role === "admin"
															? "管理员"
															: user.role === "ban"
																? "封禁"
																: "用户"}
													</span>
												</td>
												<Show when={props.canManageMute}>
													<td>
														<Show
															when={muted()}
															fallback={
																<span class="text-base-content/40 text-xs">
																	正常
																</span>
															}
														>
															<span class="flex items-center gap-1 text-error font-medium text-xs">
																{(mute()?.permanent ?? false) ? (
																	<InfinityIcon size={12} />
																) : (
																	<Clock size={12} />
																)}
																{(mute()?.permanent ?? false)
																	? "永久"
																	: props.formatRemaining(
																			mute()?.expires_at ?? null,
																		)}
															</span>
														</Show>
													</td>
												</Show>
												<td>
													<div class="flex items-center gap-1">
														<Show when={props.canUpdateUser}>
															<select
																class="select select-bordered select-xs w-20"
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
																		<option value={r}>
																			{r === "admin"
																				? "管理员"
																				: r === "ban"
																					? "封禁"
																					: "用户"}
																		</option>
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
																		class="btn btn-ghost btn-xs text-warning"
																		onClick={() => props.onStartMute(user.id)}
																	>
																		<Gavel size={13} />
																	</button>
																}
															>
																<button
																	type="button"
																	class="btn btn-ghost btn-xs text-error"
																	disabled={props.cancellingId === user.id}
																	onClick={() => props.onCancelMute(user.id)}
																>
																	<UserCheck size={13} />
																</button>
															</Show>
														</Show>
														<Show when={props.canDeleteUser && !isSelf()}>
															<button
																type="button"
																class="btn btn-ghost btn-xs text-error/60"
																onClick={() => props.onDelete(user.id)}
															>
																<Trash size={13} />
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
		</>
	);
}
