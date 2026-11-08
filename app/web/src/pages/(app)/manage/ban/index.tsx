import { Button, toast } from "@gospeak/components";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Ban, UserCheck } from "lucide-react";
import { useState } from "react";
import { listUsers, updateUserRole } from "@/api/user";
import type { BackendUser } from "@/api/auth";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import UserSearchSelect from "@/components/manage/UserSearchSelect";
import ConfirmModal from "@/components/common/ConfirmModal";
import {
	invalidateQueries,
	restoreQueries,
	runOptimistic,
	snapshotQueries,
	updateQueryData,
} from "@/utils/optimistic";
import { toastActionError } from "@/utils/toast";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";

const BAN_ROLE = "ban";
const USER_ROLE = "user";

export const Route = createFileRoute("/(app)/manage/ban/")({
	beforeLoad: () => {
		if (!hasPermission("user:update")) {
			denyAccess();
		}
	},
	component: BanPage,
	staticData: {
		title: "封禁管理",
	},
});

export default function BanPage() {
	const queryClient = useQueryClient();
	const [userSearch, setUserSearch] = useState("");
	const [userQuery, setUserQuery] = useState("");
	const { data: usersData, isLoading } = useQuery({
		queryKey: ["manage-ban-user-search", userQuery],
		queryFn: () => listUsers(1, 50, true, userQuery || undefined),
	});
	const [targetUserId, setTargetUserId] = useState<number | "">("");
	const [banning, setBanning] = useState(false);
	const [confirmBan, setConfirmBan] = useState(false);
	const [unbanningId, setUnbanningId] = useState<number | null>(null);

	const allUsers = usersData?.users || [];
	const bannedUsers = allUsers.filter((u) => u.role === BAN_ROLE);
	const normalUsers = allUsers.filter((u) => u.role !== BAN_ROLE);
	const targetUser = normalUsers.find((u) => u.id === targetUserId);

	type UserList = { users: BackendUser[]; total: number };
	const BAN_SEARCH_KEY = ["manage-ban-user-search"] as const;
	const USERS_KEY = ["manage-users"] as const;
	const snapshot = () =>
		snapshotQueries(queryClient, [
			{ queryKey: BAN_SEARCH_KEY },
			{ queryKey: USERS_KEY },
		]);
	const setRole = (uid: number, role: string) => {
		for (const key of [BAN_SEARCH_KEY, USERS_KEY]) {
			updateQueryData<UserList>(queryClient, { queryKey: key }, (prev) => ({
				...prev,
				users: prev.users.map((u) => (u.id === uid ? { ...u, role } : u)),
			}));
		}
	};
	const reconcile = () =>
		invalidateQueries(queryClient, [
			{ queryKey: BAN_SEARCH_KEY },
			{ queryKey: USERS_KEY },
		]);

	const handleBan = async () => {
		const uid = targetUserId;
		if (!uid) return;
		setBanning(true);
		try {
			await runOptimistic({
				snapshot,
				apply: () => setRole(uid, BAN_ROLE),
				restore: (snap) => restoreQueries(queryClient, snap),
				request: () => updateUserRole(uid, BAN_ROLE),
				settle: reconcile,
				onError: (error) => toastActionError(error, "封禁失败"),
				onSuccess: () => {
					setTargetUserId("");
					toast.success("用户已封禁");
				},
			});
		} finally {
			setBanning(false);
		}
	};

	const handleUnban = async (uid: number) => {
		setUnbanningId(uid);
		try {
			await runOptimistic({
				snapshot,
				apply: () => setRole(uid, USER_ROLE),
				restore: (snap) => restoreQueries(queryClient, snap),
				request: () => updateUserRole(uid, USER_ROLE),
				settle: reconcile,
				onError: (error) => toastActionError(error, "解除封禁失败"),
				onSuccess: () => toast.success("已解除封禁"),
			});
		} finally {
			setUnbanningId(null);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Ban size={20} />}
				title="封禁管理"
				description="封禁后用户无法登录与访问"
			/>

			<ManageSection
				title="已封禁用户"
				description={`${bannedUsers.length} 人`}
				padded={false}
			>
				{isLoading ? (
					<span className="border-primary border-t-primary m-4 block size-5 animate-spin rounded-full border-2" />
				) : bannedUsers.length === 0 ? (
					<div className="border-border/60 bg-muted/20 text-muted-foreground m-4 rounded-lg border border-dashed py-10 text-center text-sm">
						暂无封禁用户
					</div>
				) : (
					<div className="overflow-x-auto">
						<table className="w-full text-sm">
							<thead>
								<tr className={manageTableHeadClass}>
									<th className="px-3 py-2 text-left">ID</th>
									<th className="px-3 py-2 text-left">用户名</th>
									<th className="px-3 py-2 text-left">显示名</th>
									<th className="px-3 py-2 text-left">操作</th>
								</tr>
							</thead>
							<tbody>
								{bannedUsers.map((user) => (
									<tr key={user.id} className={manageTableRowClass}>
										<td className="text-muted-foreground px-3 py-2 font-mono text-xs">
											{user.id}
										</td>
										<td className="px-3 py-2 font-semibold">{user.name}</td>
										<td className="text-foreground px-3 py-2">
											{user.display_name || "—"}
										</td>
										<td className="px-3 py-2">
											<button
												type="button"
												className="text-muted-foreground hover:bg-muted inline-flex cursor-pointer items-center gap-1 rounded-sm px-2 py-1 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50"
												disabled={unbanningId === user.id}
												onClick={() => void handleUnban(user.id)}
											>
												<UserCheck size={14} />
												解除封禁
											</button>
										</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</ManageSection>

			<ManageSection title="添加封禁" description="将用户角色设为 ban">
				<div className="flex flex-wrap items-end gap-3">
					<UserSearchSelect
						id="ban-user"
						value={targetUserId}
						onChange={setTargetUserId}
						users={normalUsers}
						loading={isLoading}
						disabled={banning}
						searchValue={userSearch}
						onSearchInput={setUserSearch}
						onSearch={() => setUserQuery(userSearch.trim())}
					/>
					<Button
						type="button"
						variant="destructive"
						disabled={!targetUserId || banning}
						onClick={() => setConfirmBan(true)}
					>
						<Ban size={16} />
						确认封禁
					</Button>
				</div>
			</ManageSection>

			<ConfirmModal
				open={confirmBan}
				title="封禁用户"
				message={
					<span>
						确认封禁「
						{targetUser?.display_name || targetUser?.name || `#${targetUserId}`}
						」？封禁后该用户无法登录与访问，可在上方列表解除。
					</span>
				}
				confirmText="封禁"
				loading={banning}
				onClose={() => setConfirmBan(false)}
				onConfirm={() => {
					setConfirmBan(false);
					void handleBan();
				}}
			/>
		</ManagePage>
	);
}
