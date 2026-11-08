import { Checkbox, toast } from "@gospeak/components";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Plus, Save, ShieldCheck, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";
import { listPermissions } from "@/api/permission";
import {
	createRole,
	deleteRole,
	getRolePermissions,
	listRoles,
	syncRolePermissions,
	type Role,
} from "@/api/role";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import ConfirmModal from "@/components/common/ConfirmModal";
import { denyAccess } from "@/utils/accessDenied";
import {
	clearBackgroundFailure,
	reportBackgroundFailure,
} from "@/utils/backgroundFailure";
import {
	invalidateQueries,
	restoreQueries,
	runOptimistic,
	snapshotQueries,
	updateQueryData,
} from "@/utils/optimistic";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";

const SYSTEM_ROLES = new Set(["admin", "user", "ban"]);

export const Route = createFileRoute("/(app)/manage/roles/")({
	beforeLoad: () => {
		if (!hasPermission("role:read")) {
			denyAccess();
		}
	},
	component: RolesPage,
	staticData: {
		title: "角色权限",
	},
});

function roleLabel(name: string) {
	if (name === "admin") return "管理员";
	if (name === "ban") return "封禁";
	if (name === "user") return "普通用户";
	return name;
}

function RolesPage() {
	const queryClient = useQueryClient();
	const canManage = hasPermission("role:manage");
	const { data: roles, isLoading: rolesLoading } = useQuery({
		queryKey: ["manage-roles"],
		queryFn: listRoles,
	});
	const { data: permissions = [] } = useQuery({
		queryKey: ["manage-permissions"],
		queryFn: listPermissions,
	});

	const [selectedRole, setSelectedRole] = useState("");
	const [selectedCodes, setSelectedCodes] = useState<Set<string>>(new Set());
	const [loadingPerms, setLoadingPerms] = useState(false);
	const [permsError, setPermsError] = useState("");
	const [permsReload, setPermsReload] = useState(0);
	const [saving, setSaving] = useState(false);
	const [newRoleName, setNewRoleName] = useState("");
	const [creating, setCreating] = useState(false);
	const [deletingRole, setDeletingRole] = useState<string | null>(null);
	const [pendingDeleteRole, setPendingDeleteRole] = useState<Role | null>(null);

	const roleList: Role[] = roles ?? [];

	useEffect(() => {
		if (!roleList.length) return;
		if (!roleList.some((r) => r.name === selectedRole)) {
			setSelectedRole(roleList[0].name);
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [roleList.length]);

	useEffect(() => {
		const role = selectedRole;
		if (!role) return;
		setLoadingPerms(true);
		setPermsError("");
		getRolePermissions(role)
			.then((data) => {
				clearBackgroundFailure(`role-perms:${role}`);
				setSelectedCodes(new Set<string>(data.permissions));
			})
			.catch((error) => {
				// 原来静默清空勾选，用户会误以为「这个角色没有权限」并保存，
				// 从而把权限真的清掉。失败时必须保留空勾选以外的明确错误态。
				setSelectedCodes(new Set<string>());
				setPermsError(
					reportBackgroundFailure(
						`role-perms:${role}`,
						error,
						"角色权限加载失败，请重新选择角色重试",
					),
				);
			})
			.finally(() => setLoadingPerms(false));
	}, [selectedRole, permsReload]);

	const toggle = (code: string, checked: boolean) => {
		setSelectedCodes((prev) => {
			const next = new Set(prev);
			if (checked) next.add(code);
			else next.delete(code);
			return next;
		});
	};

	const ROLES_KEY = ["manage-roles"] as const;
	const reconcile = () =>
		invalidateQueries(queryClient, [{ queryKey: ROLES_KEY }]);

	const handleSave = async () => {
		const role = selectedRole;
		if (!role || !canManage) return;
		setSaving(true);
		try {
			await syncRolePermissions(role, Array.from(selectedCodes));
			toast.success("权限已保存");
		} catch (error) {
			toastActionError(error, "保存权限失败");
		} finally {
			setSaving(false);
		}
	};

	const handleCreate = async () => {
		const name = newRoleName.trim();
		if (!name || !canManage) return;
		setCreating(true);
		const now = new Date().toISOString();
		const optimisticRole: Role = {
			id: -1,
			name,
			created_at: now,
			updated_at: now,
		};
		try {
			await runOptimistic({
				snapshot: () => snapshotQueries(queryClient, [{ queryKey: ROLES_KEY }]),
				apply: () =>
					updateQueryData<Role[]>(
						queryClient,
						{ queryKey: ROLES_KEY },
						(prev) =>
							prev.some((r) => r.name === name)
								? prev
								: [...prev, optimisticRole],
					),
				restore: (snap) => restoreQueries(queryClient, snap),
				request: () => createRole(name),
				settle: reconcile,
				onError: (error) => toastActionError(error, "创建角色失败"),
				onSuccess: () => {
					setNewRoleName("");
					setSelectedRole(name);
					toast.success(`角色「${name}」已创建`);
				},
			});
		} finally {
			setCreating(false);
		}
	};

	const handleDelete = async (role: Role) => {
		if (!canManage) return;
		setPendingDeleteRole(role);
	};

	const confirmDelete = async () => {
		const role = pendingDeleteRole;
		if (!role) return;
		setDeletingRole(role.name);
		try {
			await runOptimistic({
				snapshot: () => snapshotQueries(queryClient, [{ queryKey: ROLES_KEY }]),
				apply: () =>
					updateQueryData<Role[]>(
						queryClient,
						{ queryKey: ROLES_KEY },
						(prev) => prev.filter((r) => r.id !== role.id),
					),
				restore: (snap) => restoreQueries(queryClient, snap),
				request: () => deleteRole(role.id),
				settle: reconcile,
				onError: (error) => toastActionError(error, "删除角色失败"),
				onSuccess: () => {
					toast.success(`角色「${roleLabel(role.name)}」已删除`);
					setPendingDeleteRole(null);
				},
			});
		} finally {
			setDeletingRole(null);
		}
	};

	const btnXs =
		"bg-primary text-primary-foreground hover:opacity-90 inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm px-2 text-xs transition-opacity disabled:pointer-events-none disabled:opacity-50";

	return (
		<ManagePage>
			<ManageHeader
				icon={<ShieldCheck size={20} />}
				title="角色权限"
				description="管理平台角色及其权限码"
			/>

			{rolesLoading ? (
				<ManageLoading label="正在加载角色..." />
			) : (
				<div className="grid min-w-0 gap-4 md:grid-cols-[240px_minmax(0,1fr)]">
					<ManageSection
						title="角色"
						actions={
							canManage ? (
								<div className="flex items-center gap-1.5">
									<input
										type="text"
										className="border-input bg-card text-foreground focus-visible:ring-ring placeholder:text-faint-foreground flex h-7 w-28 rounded-sm border px-2 text-xs focus-visible:outline-none focus-visible:ring-2"
										placeholder="新角色名"
										value={newRoleName}
										onChange={(e) => setNewRoleName(e.currentTarget.value)}
										onKeyDown={(e) => {
											if (e.key === "Enter") void handleCreate();
										}}
									/>
									<button
										type="button"
										className={btnXs}
										disabled={creating}
										onClick={() => void handleCreate()}
									>
										<Plus size={14} />
									</button>
								</div>
							) : undefined
						}
					>
						<div className="flex flex-col gap-1">
							{roleList.length === 0 && (
								<p className="text-faint-foreground px-3 py-2 text-sm">
									暂无角色
								</p>
							)}
							{roleList.map((role) => (
								<div key={role.name} className="flex items-center gap-1">
									<button
										type="button"
										className={`hover:bg-muted flex flex-1 cursor-pointer items-center justify-start gap-2 rounded-sm px-3 py-2 text-left text-sm transition-colors ${selectedRole === role.name ? "bg-muted" : ""}`}
										onClick={() => setSelectedRole(role.name)}
									>
										<span className="truncate">{roleLabel(role.name)}</span>
										{SYSTEM_ROLES.has(role.name) && (
											<span className="bg-muted text-muted-foreground inline-flex items-center rounded-full px-1.5 text-2xs">
												系统
											</span>
										)}
									</button>
									{canManage && !SYSTEM_ROLES.has(role.name) && (
										<button
											type="button"
											className="text-destructive hover:bg-destructive/10 inline-flex size-7 cursor-pointer items-center justify-center rounded-sm transition-colors disabled:pointer-events-none disabled:opacity-50"
											disabled={deletingRole === role.name}
											onClick={() => void handleDelete(role)}
											aria-label={`删除角色 ${roleLabel(role.name)}`}
										>
											<Trash2 size={14} />
										</button>
									)}
								</div>
							))}
						</div>
					</ManageSection>

					<ManageSection
						title={`权限 — ${roleLabel(selectedRole)}`}
						actions={
							canManage ? (
								<button
									type="button"
									className={btnXs}
									disabled={saving || loadingPerms || !!permsError}
									onClick={() => void handleSave()}
								>
									<Save size={14} />
									保存
								</button>
							) : undefined
						}
					>
						{permsError && (
							<div
								role="alert"
								className="border-destructive/40 bg-destructive/10 text-destructive mb-2 flex flex-wrap items-center gap-2 rounded-sm border px-3 py-2 text-xs"
							>
								<span>{permsError}（已禁用保存，避免把权限误清空）</span>
								<button
									type="button"
									className="underline underline-offset-2"
									onClick={() => setPermsReload((n) => n + 1)}
								>
									重试
								</button>
							</div>
						)}
						{loadingPerms ? (
							<ManageLoading label="正在加载权限..." />
						) : permissions.length === 0 ? (
							<p className="text-faint-foreground px-3 py-2 text-sm">
								暂无可分配的权限
							</p>
						) : (
							<div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
								{permissions.map((perm) => (
									<label
										key={perm.code}
										className="border-border/70 hover:bg-muted/40 flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors"
									>
										<Checkbox
											checked={selectedCodes.has(perm.code)}
											onCheckedChange={(checked) =>
												toggle(perm.code, checked === true)
											}
											disabled={!canManage}
										/>
										<span className="min-w-0">
											<span className="block truncate font-medium">
												{perm.name}
											</span>
											<span className="text-faint-foreground block truncate text-2xs">
												{perm.code}
											</span>
										</span>
									</label>
								))}
							</div>
						)}
					</ManageSection>
				</div>
			)}

			<ConfirmModal
				open={!!pendingDeleteRole}
				title="删除角色"
				message={
					<span>
						确认删除角色「
						{pendingDeleteRole ? roleLabel(pendingDeleteRole.name) : ""}
						」？该角色下已授予的权限会一并移除，此操作不可恢复。
					</span>
				}
				confirmText="删除"
				loading={!!deletingRole}
				onClose={() => setPendingDeleteRole(null)}
				onConfirm={confirmDelete}
			/>
		</ManagePage>
	);
}
