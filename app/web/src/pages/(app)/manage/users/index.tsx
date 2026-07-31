import { createFileRoute, redirect } from "@tanstack/solid-router";
import Users from "lucide-solid/icons/users";
import { createResource, createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { cancelMute, createMute, listMutes, type MuteRecord } from "@/api/mute";
import { deleteUser, listUsers, updateUserRole } from "@/api/user";
import { ManageHeader, ManagePage } from "@/components/manage/ManageShell";
import userStore from "@/stores/userStore";
import { formatRemaining } from "@/utils/format";
import { hasPermission } from "@/utils/permissions";
import ActiveMuteList from "./components/ActiveMuteList";
import QuickMuteForm from "./components/QuickMuteForm";
import UsersTable from "./components/UsersTable";

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
		} catch (e: any) {}
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
		} catch (e: any) {}
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
		<ManagePage>
			<ManageHeader
				icon={<Users size={18} />}
				title="用户管理"
				description="查看用户、调整角色，并快速处理禁言"
			/>

			<UsersTable
				loading={_usersData.loading}
				users={users()}
				roles={ROLES}
				muteMap={_muteMap()}
				canManageMute={canManageMute()}
				canUpdateUser={canUpdateUser()}
				canDeleteUser={canDeleteUser()}
				selfId={userStore.user()?.id}
				cancellingId={cancellingId()}
				formatRemaining={formatRemaining}
				onRoleChange={(userId, role) => void handleRoleChange(userId, role)}
				onStartMute={(userId) => {
					setMuteUserId(userId);
					setMutePerm(false);
					setMuteDuration(3600);
				}}
				onCancelMute={(userId) => void handleCancelMute(userId)}
				onDelete={(userId) => void handleDeleteUser(userId)}
			/>

			<Show when={canManageMute()}>
				<QuickMuteForm
					users={users()}
					userMap={userMap()}
					muteUserId={muteUserId()}
					muteDuration={muteDuration()}
					mutePerm={mutePerm()}
					muteReason={muteReason()}
					submitting={submitting()}
					setMuteUserId={setMuteUserId}
					setMuteDuration={setMuteDuration}
					setMutePerm={setMutePerm}
					setMuteReason={setMuteReason}
					onSubmit={() => void handleMute()}
				/>

				<ActiveMuteList
					loading={mutes.loading}
					mutes={mutes() ?? []}
					userMap={userMap()}
					cancellingId={cancellingId()}
					formatRemaining={formatRemaining}
					onCancel={(userId) => void handleCancelMute(userId)}
				/>
			</Show>
		</ManagePage>
	);
}
