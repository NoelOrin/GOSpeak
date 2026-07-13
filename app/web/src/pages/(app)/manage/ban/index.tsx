import { createFileRoute, redirect } from "@tanstack/solid-router";
import Ban from "lucide-solid/icons/ban";
import UserCheck from "lucide-solid/icons/user-check";
import Users from "lucide-solid/icons/users";
import { createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { listUsers, updateUserRole } from "@/api/user";
import userStore from "@/stores/userStore";

const BAN_ROLE = "ban";
const USER_ROLE = "user";

export const Route = createFileRoute("/(app)/manage/ban/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
	},
	component: BanPage,
	staticData: {
		title: "封禁管理",
		icon: "icon-manage",
	},
});

function BanPage() {
	const [usersData, { refetch }] = createResource(() => listUsers(1, 200));
	const [targetUserId, setTargetUserId] = createSignal<number | "">("");
	const [banning, setBanning] = createSignal(false);
	const [unbanningId, setUnbanningId] = createSignal<number | null>(null);

	const allUsers = () => usersData()?.users || [];
	const bannedUsers = () => allUsers().filter((u) => u.role === BAN_ROLE);
	const _normalUsers = () => allUsers().filter((u) => u.role !== BAN_ROLE);

	const handleBan = async () => {
		const uid = targetUserId();
		if (!uid) {
			showToast("请选择用户", { type: "warning" });
			return;
		}
		setBanning(true);
		try {
			await updateUserRole(uid, BAN_ROLE);
			showToast("用户已被封禁", { type: "success" });
			setTargetUserId("");
			refetch();
		} catch (e: any) {
			showToast(e?.message || "封禁失败", { type: "error" });
		} finally {
			setBanning(false);
		}
	};

	const handleUnban = async (uid: number) => {
		setUnbanningId(uid);
		try {
			await updateUserRole(uid, USER_ROLE);
			showToast("封禁已解除", { type: "success" });
			refetch();
		} catch (e: any) {
			showToast(e?.message || "解除失败", { type: "error" });
		} finally {
			setUnbanningId(null);
		}
	};

	return (
		<div class="flex h-full min-h-0 flex-col gap-4 p-4">
			<div class="flex items-center gap-2 mb-3">
				<Ban size={20} />
				<h3 class="font-bold text-lg">封禁管理</h3>
			</div>
			{/* ========== 已封禁用户 ========== */}
			<div>
				<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
					<Ban size={16} class="text-error" />
					<span>已封禁用户</span>
					<span class="text-base-content/50 text-xs">
						({bannedUsers().length} 人)
					</span>
				</div>
				<Show
					when={!usersData.loading}
					fallback={<div class="loading loading-spinner loading-sm" />}
				>
					<Show
						when={bannedUsers().length > 0}
						fallback={
							<div class="text-base-content/50 py-8 text-center text-sm">
								暂无封禁用户
							</div>
						}
					>
						<div class="overflow-x-auto">
							<table class="table table-zebra table-sm">
								<thead>
									<tr>
										<th>ID</th>
										<th>用户名</th>
										<th>显示名</th>
										<th>操作</th>
									</tr>
								</thead>
								<tbody>
									<For each={bannedUsers()}>
										{(user) => (
											<tr>
												<td class="font-mono text-xs">{user.id}</td>
												<td class="font-medium text-error">{user.name}</td>
												<td>{user.display_name || "—"}</td>
												<td>
													<button
														type="button"
														class="btn btn-ghost btn-xs text-success"
														disabled={unbanningId() === user.id}
														onClick={() => handleUnban(user.id)}
													>
														<UserCheck size={14} />
														解除封禁
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

			<div class="border-base-300 border-t" />

			{/* ========== 封禁用户 ========== */}
			<div>
				<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
					<Users size={16} />
					<span>封禁用户</span>
					<span class="text-base-content/50 text-xs">
						（将用户角色设为 ban，该用户将无法登录）
					</span>
				</div>
				<div class="flex items-end gap-3">
					<div class="form-control">
						<label class="label py-1" for="ban-select-user">
							<span class="label-text text-xs">选择用户</span>
						</label>
						<select
							id="ban-select-user"
							class="select select-bordered select-sm w-52"
							value={targetUserId()}
							onChange={(e) =>
								setTargetUserId(
									e.currentTarget.value ? Number(e.currentTarget.value) : "",
								)
							}
						>
							<option value="">选择用户</option>
							<For each={_normalUsers()}>
								{(u) => (
									<option value={u.id}>
										{u.display_name || u.name} ({u.name})
									</option>
								)}
							</For>
						</select>
					</div>
					<button
						type="button"
						class="btn btn-error btn-sm gap-2"
						disabled={!targetUserId() || banning()}
						onClick={handleBan}
					>
						<Ban size={15} />
						确认封禁
					</button>
				</div>
			</div>
		</div>
	);
}
