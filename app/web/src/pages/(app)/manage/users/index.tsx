import { createFileRoute, redirect } from "@tanstack/solid-router";
import userStore from "@/stores/userStore";
import {
	createResource,
	createSignal,
	For,
	Show,
} from "solid-js";
import { showToast } from "solid-notifications";
import Users from "lucide-solid/icons/users";
import Trash from "lucide-solid/icons/trash";
import Gavel from "lucide-solid/icons/gavel";
import UserCheck from "lucide-solid/icons/user-check";
import Clock from "lucide-solid/icons/clock";
import Infinity from "lucide-solid/icons/infinity";
import UserX from "lucide-solid/icons/user-x";
import { listUsers, updateUserRole, deleteUser } from "@/api/user";
import { createMute, cancelMute, listMutes, type MuteRecord } from "@/api/mute";
import { formatRemaining } from "@/utils/format";

const ROLES = ["user", "admin", "ban"];

export const Route = createFileRoute("/(app)/manage/users/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
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
	const [usersData, { refetch: refetchUsers }] = createResource(
		() => listUsers(1, 200),
	);
	const [mutes, { refetch: refetchMutes }] = createResource(() => listMutes());

	const muteMap = () => {
		const data = mutes();
		if (!data) return new Map<number, MuteRecord>();
		const m = new Map<number, MuteRecord>();
		for (const mute of data) {
			m.set(mute.user_id, mute);
		}
		return m;
	};

	// Role update
	const roleActions = () => {
		const currentUser = userStore.user();
		return ROLES.map((r) => ({
			value: r,
			label: r === "admin" ? "管理员" : "普通用户",
		}));
	};

	const handleRoleChange = async (userId: number, newRole: string) => {
		try {
			await updateUserRole(userId, newRole);
			showToast("角色已更新", { type: "success" });
			refetchUsers();
		} catch (e: any) {
			showToast(e?.message || "更新失败", { type: "error" });
		}
	};

	const handleDeleteUser = async (userId: number) => {
		if (!confirm("确认删除该用户？此操作不可恢复。")) return;
		try {
			await deleteUser(userId);
			showToast("用户已删除", { type: "success" });
			refetchUsers();
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
			refetchMutes();
		} catch (e: any) {
			showToast(e?.message || "禁言失败", { type: "error" });
		} finally {
			setSubmitting(false);
		}
	};

	const handleCancelMute = async (uid: number) => {
		setCancellingId(uid);
		try {
			await cancelMute(uid);
			showToast("禁言已解除", { type: "success" });
			refetchMutes();
		} catch (e: any) {
			showToast(e?.message || "解除失败", { type: "error" });
		} finally {
			setCancellingId(null);
		}
	};

	const users = () => usersData()?.users || [];
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
					when={!usersData.loading}
					fallback={<div class="loading loading-spinner loading-sm py-8 mx-auto block" />}
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
									<th>禁言</th>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								<For each={users()}>
									{(user) => {
										const mute = () => muteMap().get(user.id);
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
														{user.role === "admin" ? "管理员" : user.role === "ban" ? "封禁" : "用户"}
													</span>
												</td>
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
															{mute()!.permanent ? (
																<Infinity size={12} />
															) : (
																<Clock size={12} />
															)}
															{mute()!.permanent
																? "永久"
																: formatRemaining(mute()!.expires_at)}
														</span>
													</Show>
												</td>
												<td>
													<div class="flex items-center gap-1">
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
																		{r === "admin" ? "管理员" : r === "ban" ? "封禁" : "用户"}
																	</option>
																)}
															</For>
														</select>
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
														<Show when={!isSelf()}>
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

			<div class="border-base-300 border-t" />

			{/* ========== 快速禁言 ========== */}
			<div>
				<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
					<Gavel size={16} />
					<span>快速禁言</span>
					<Show when={muteUserId()}>
						<span class="text-primary text-xs">
							目标: {userMap().get(muteUserId() as number) || `#${muteUserId()}`}
						</span>
					</Show>
				</div>
				<div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
					<div class="form-control">
						<label class="label py-1">
							<span class="label-text text-xs">用户</span>
						</label>
						<select
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
						<label class="label py-1">
							<span class="label-text text-xs">类型</span>
						</label>
						<div class="flex items-center gap-3 pt-1">
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
							<label class="label py-1">
								<span class="label-text text-xs">时长（秒）</span>
							</label>
							<input
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
						<label class="label py-1">
							<span class="label-text text-xs">原因</span>
						</label>
						<input
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
															<Infinity size={13} />
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
														onClick={() =>
															handleCancelMute(mute.user_id)
														}
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
		</div>
	);
}
