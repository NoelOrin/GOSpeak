import { createFileRoute, redirect } from "@tanstack/solid-router";
import Ban from "lucide-solid/icons/ban";
import UserCheck from "lucide-solid/icons/user-check";
import { createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { listUsers, updateUserRole } from "@/api/user";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import UserSearchSelect from "@/components/manage/UserSearchSelect";
import { hasPermission } from "@/utils/permissions";

const BAN_ROLE = "ban";
const USER_ROLE = "user";

export const Route = createFileRoute("/(app)/manage/ban/")({
	beforeLoad: () => {
		if (!hasPermission("user:update")) {
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
	const [userSearch, setUserSearch] = createSignal("");
	const [userQuery, setUserQuery] = createSignal("");
	const [usersData, { refetch }] = createResource(
		() => userQuery().trim(),
		(keyword) => listUsers(1, 50, true, keyword || undefined),
	);
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
		} catch {
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
		} catch {
		} finally {
			setUnbanningId(null);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Ban size={18} />}
				title="封禁管理"
				description="封禁后用户无法登录与访问"
			/>

			<ManageSection
				title="已封禁用户"
				description={`${bannedUsers().length} 人`}
				padded={false}
			>
				<Show
					when={!usersData.loading}
					fallback={<div class="loading loading-spinner loading-sm m-4" />}
				>
					<Show
						when={bannedUsers().length > 0}
						fallback={
							<div class="m-4 rounded-xl border border-dashed border-base-300 bg-base-200/20 py-10 text-center text-sm text-base-content/55">
								暂无封禁用户
							</div>
						}
					>
						<div class="overflow-x-auto">
							<table class="table table-sm">
								<thead>
									<tr class={manageTableHeadClass}>
										<th>ID</th>
										<th>用户名</th>
										<th>显示名</th>
										<th>操作</th>
									</tr>
								</thead>
								<tbody>
									<For each={bannedUsers()}>
										{(user) => (
											<tr class={manageTableRowClass}>
												<td class="font-mono text-xs text-base-content/70">
													{user.id}
												</td>
												<td class="font-semibold">{user.name}</td>
												<td class="text-base-content/75">
													{user.display_name || "—"}
												</td>
												<td>
													<button
														type="button"
														class="btn btn-ghost btn-xs text-base-content/60"
														disabled={unbanningId() === user.id}
														onClick={() => void handleUnban(user.id)}
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
			</ManageSection>

			<ManageSection title="添加封禁" description="将用户角色设为 ban">
				<div class="flex flex-wrap items-end gap-3">
					<UserSearchSelect
						id="ban-user"
						value={targetUserId()}
						onChange={setTargetUserId}
						users={_normalUsers()}
						loading={usersData.loading}
						disabled={banning()}
						searchValue={userSearch()}
						onSearchInput={setUserSearch}
						onSearch={() => setUserQuery(userSearch().trim())}
					/>
					<button
						type="button"
						class="btn btn-sm border border-base-300 bg-base-100 text-base-content shadow-none hover:bg-base-200"
						disabled={!targetUserId() || banning()}
						onClick={() => void handleBan()}
					>
						<Ban size={15} />
						确认封禁
					</button>
				</div>
			</ManageSection>
		</ManagePage>
	);
}
