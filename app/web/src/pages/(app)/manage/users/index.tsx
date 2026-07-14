import { createFileRoute, redirect } from "@tanstack/solid-router";
import Clock from "lucide-solid/icons/clock";
import Gavel from "lucide-solid/icons/gavel";
import InfinityIcon from "lucide-solid/icons/infinity";
import Trash from "lucide-solid/icons/trash";
import UserCheck from "lucide-solid/icons/user-check";
import UserX from "lucide-solid/icons/user-x";
import Users from "lucide-solid/icons/users";
import { createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { cancelMute, createMute, listMutes, type MuteRecord } from "@/api/mute";
import { deleteUser, listUsers, updateUserRole } from "@/api/user";
import userStore from "@/stores/userStore";
import { formatRemaining } from "@/utils/format";
import { hasPermission } from "@/utils/permissions";

const ROLES = ["user", "admin", "ban"];

const canUpdateUser = () => hasPermission("user:update");
const canDeleteUser = () => hasPermission("user:delete");
const canManageMute = () => hasPermission("mute:manage");

export const Route = createFileRoute("/(app)/manage/users/")({
	beforeLoad: () => {
		if (!hasPermission("user:read")) {
			throw redirect({ to: "/" });
		}
	},
	component: UsersPage,
	staticData: {
		title: "用户管理",
		icon: "icon-manage",
	},
});

function UsersPage() {
	const [_usersData, { refetch: _refetchUsers }] = createResource(() =>
		listUsers(1, 200),
	);
	const [mutes, { refetch: _refetchMutes }] = createResource(
		() => canManageMute(),
		async (enabled) => {
			if (!enabled) return [] as MuteRecord[];
			return listMutes();
		},
	);

	const _muteMap = () => {
		const data = mutes();
		if (!data) return new Map<number, MuteRecord>();
		const m = new Map<number, MuteRecord>();
		for (const mute of data) {
			m.set(mute.user_id, mute);
		}
		return m;
	};

	const handleRoleChange = async (userId: number, newRole: string) => {
		if (!canUpdateUser()) {
			showToast("无修改角色权限", { type: "error" });
			return;
		}
		try {
			await updateUserRole(userId, newRole);
			showToast("角色已更新", { type: "success" });
			_refetchUsers();
		} catch (e: any) {
			showToast(e?.message || "更新失败", { type: "error" });
		}
	};

	const handleDeleteUser = async (userId: number) => {
		if (!canDeleteUser()) {
			showToast("无删除用户权限", { type: "error" });
			return;
		}
		if (!confirm("确认删除该用户？此操作不可恢复。")) return;
		try {
			await deleteUser(userId);
			showToast("用户已删除", { type: "success" });
			_refetchUsers();
		} catch (e: any) {
			showToast(e?.message || "删除失败", { type: "error" });
		}
	};

	// Mute management
	const [muteUserId, setMuteUserId] = createSignal<number | "">("");
	const [muteDuration, setMuteDuration] = createSignal(3600);
	const [mutePerm, setMutePerm] = createSignal(false);
	const [muteReason, setMuteReason] = createSignal("");
	const [submitting, setSubmitting] = createSignal(false);
	const [cancellingId, setCancellingId] = createSignal<number | null>(null);

	const handleMute = async () => {
		if (!canManageMute()) {
			showToast("无禁言管理权限", { type: "error" });
			return;
		}
		const uid = muteUserId();
		if (!uid) {
			showToast("请选择用户", { type: "warning" });
			return;
		}
		if (!mutePerm() && muteDuration() <= 0) {
			showToast("请输入有效时长", { type: "warning" });
			return;
		}
		setSubmitting(true);
		try {
			await createMute({
				user_id: uid,
				duration: muteDuration(),
				permanent: mutePerm(),
				reason: muteReason(),
			});
			showToast("禁言已生效", { type: "success" });
			setMuteUserId("");
			setMuteDuration(3600);
			setMutePerm(false);
			setMuteReason("");
			_refetchMutes();
		} catch (e: any) {
			showToast(e?.message || "禁言失败", { type: "error" });
		} finally {
			setSubmitting(false);
		}
	};

	const handleCancelMute = async (uid: number) => {
		if (!canManageMute()) {
			showToast("无禁言管理权限", { type: "error" });
			return;
		}
		setCancellingId(uid);
		try {
			await cancelMute(uid);
			showToast("禁言已解除", { type: "success" });
			_refetchMutes();
		} catch (e: any) {
			showToast(e?.message || "解除失败", { type: "error" });
		} finally {
			setCancellingId(null);
		}
	};

	const users = () => (_usersData()?.users || []).filter((u) => !u.is_bot);
	const userMap = () => {
		const m = new Map<number, string>();
		for (const u of users()) {
			m.set(u.id, `${u.display_name || u.name}`);
		}
		return m;
	};

	return (
		<div class="flex h-full min-h-0 flex-col gap-4 p-4">
			{/* ========== 用户列表 ========== */}
			<div class="flex items-center justify-between gap-3">
				<div class="flex items-center gap-2">
					<Users size={18} />
					<h3 class="font-bold text-lg">用户列表</h3>
					<span class="text-base-content/50 text-xs">
						({users().length} 人)
					</span>
				</div>
			</div>

			<div class="overflow-x-auto">
				<Show
					when={!_usersData.loading}
					fallback={
						<div class="loading loading-spinner loading-sm py-8 mx-auto block" />
					}
				>
					<Show
						when={users().length > 0}
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
									<Show when={canManageMute()}>
										<th>禁言</th>
									</Show>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								<For each={users()}>
									{(user) => {
										const mute = () => _muteMap().get(user.id);
										const muted = () => !!mute();
										const isSelf = () => userStore.user()?.id === user.id;
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
												<Show when={canManageMute()}>
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
																	: formatRemaining(mute()?.expires_at ?? null)}
															</span>
														</Show>
													</td>
												</Show>
												<td>
													<div class="flex items-center gap-1">
														{/* 改角色：user:update */}
														<Show when={canUpdateUser()}>
															<select
																class="select select-bordered select-xs w-20"
																value={user.role}
																disabled={isSelf()}
																onChange={(e) =>
																	handleRoleChange(
																		user.id,
																		e.currentTarget.value,
																	)
																}
															>
																<For each={ROLES}>
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
														{/* 行内禁言：mute:manage */}
														<Show when={canManageMute()}>
															<Show
																when={muted()}
																fallback={
																	<button
																		type="button"
																		class="btn btn-ghost btn-xs text-warning"
																		onClick={() => {
																			setMuteUserId(user.id);
																			setMutePerm(false);
																			setMuteDuration(3600);
																		}}
																	>
																		<Gavel size={13} />
																	</button>
																}
															>
																<button
																	type="button"
																	class="btn btn-ghost btn-xs text-error"
																	disabled={cancellingId() === user.id}
																	onClick={() => handleCancelMute(user.id)}
																>
																	<UserCheck size={13} />
																</button>
															</Show>
														</Show>
														{/* 删除：user:delete */}
														<Show when={canDeleteUser() && !isSelf()}>
															<button
																type="button"
																class="btn btn-ghost btn-xs text-error/60"
																onClick={() => handleDeleteUser(user.id)}
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

			<Show when={canManageMute()}>
				<div class="border-base-300 border-t" />

				{/* ========== 快速禁言 ========== */}
				<div>
					<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
						<Gavel size={16} />
						<span>快速禁言</span>
						<Show when={muteUserId()}>
							<span class="text-primary text-xs">
								目标:{" "}
								{userMap().get(muteUserId() as number) || `#${muteUserId()}`}
							</span>
						</Show>
					</div>
					<div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
						<div class="form-control">
							<label class="label py-1" for="mute-user">
								<span class="label-text text-xs">用户</span>
							</label>
							<select
								id="mute-user"
								class="select select-bordered select-sm"
								value={muteUserId()}
								onChange={(e) =>
									setMuteUserId(
										e.currentTarget.value ? Number(e.currentTarget.value) : "",
									)
								}
							>
								<option value="">选择用户</option>
								<For each={users()}>
									{(u) => (
										<option value={u.id}>
											{u.display_name || u.name} ({u.name})
										</option>
									)}
								</For>
							</select>
						</div>

						<div class="form-control">
							<label class="label py-1" for="None">
								<span class="label-text text-xs">类型</span>
							</label>
							<div id="None" class="flex items-center gap-3 pt-1">
								<label class="flex items-center gap-1.5 text-xs">
									<input
										type="radio"
										name="mute-type"
										class="radio radio-xs"
										checked={!mutePerm()}
										onChange={() => setMutePerm(false)}
									/>
									定时
								</label>
								<label class="flex items-center gap-1.5 text-xs">
									<input
										type="radio"
										name="mute-type"
										class="radio radio-xs"
										checked={mutePerm()}
										onChange={() => setMutePerm(true)}
									/>
									永久
								</label>
							</div>
						</div>

						<Show when={!mutePerm()}>
							<div class="form-control">
								<label class="label py-1" for="mute-duration">
									<span class="label-text text-xs">时长（秒）</span>
								</label>
								<input
									id="mute-duration"
									type="number"
									class="input input-bordered input-sm"
									value={muteDuration()}
									onInput={(e) =>
										setMuteDuration(Number(e.currentTarget.value) || 0)
									}
									min={1}
								/>
							</div>
						</Show>

						<div class="form-control">
							<label class="label py-1" for="mute-reason">
								<span class="label-text text-xs">原因</span>
							</label>
							<input
								id="mute-reason"
								type="text"
								class="input input-bordered input-sm"
								placeholder="违规发言"
								value={muteReason()}
								onInput={(e) => setMuteReason(e.currentTarget.value)}
							/>
						</div>
					</div>

					<div class="mt-3 flex justify-end">
						<button
							type="button"
							class="btn btn-primary btn-sm gap-2"
							disabled={!muteUserId() || submitting()}
							onClick={handleMute}
						>
							<Gavel size={15} />
							确认禁言
						</button>
					</div>
				</div>

				<div class="border-base-300 border-t" />

				{/* ========== 活跃禁言列表 ========== */}
				<div>
					<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
						<UserX size={16} />
						<span>活跃禁言</span>
						<span class="text-base-content/50 text-xs">
							({mutes()?.length || 0} 条)
						</span>
					</div>
					<Show
						when={!mutes.loading}
						fallback={<div class="loading loading-spinner loading-sm" />}
					>
						<Show
							when={(mutes()?.length || 0) > 0}
							fallback={
								<div class="text-base-content/50 py-4 text-center text-sm">
									暂无活跃禁言
								</div>
							}
						>
							<div class="overflow-x-auto">
								<table class="table table-zebra table-xs">
									<thead>
										<tr>
											<th>用户</th>
											<th>类型</th>
											<th>剩余时间</th>
											<th>原因</th>
											<th>操作者</th>
											<th>操作</th>
										</tr>
									</thead>
									<tbody>
										<For each={mutes()}>
											{(mute) => (
												<tr>
													<td>
														{userMap().get(mute.user_id) || `#${mute.user_id}`}
													</td>
													<td>
														{mute.permanent ? (
															<span class="flex items-center gap-1 text-error font-medium text-xs">
																<InfinityIcon size={13} />
																永久
															</span>
														) : (
															<span class="flex items-center gap-1 text-warning font-medium text-xs">
																<Clock size={13} />
																定时
															</span>
														)}
													</td>
													<td class="font-mono text-xs">
														{mute.permanent
															? "永久"
															: formatRemaining(mute.expires_at)}
													</td>
													<td class="max-w-40 truncate text-xs">
														{mute.reason || "—"}
													</td>
													<td class="text-xs">
														{userMap().get(mute.muter_id) ||
															`#${mute.muter_id}`}
													</td>
													<td>
														<button
															type="button"
															class="btn btn-ghost btn-xs text-error"
															disabled={cancellingId() === mute.user_id}
															onClick={() => handleCancelMute(mute.user_id)}
														>
															<UserCheck size={14} />
															解除
														</button>
													</td>
												</tr>
											)}
										</For>
									</tbody>
								</table>
							</div>
						</Show>
					</Show>
				</div>
			</Show>
		</div>
	);
}
