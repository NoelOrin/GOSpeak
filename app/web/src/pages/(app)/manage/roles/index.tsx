import { createFileRoute, redirect } from "@tanstack/solid-router";
import ShieldCheck from "lucide-solid/icons/shield-check";
import Plus from "lucide-solid/icons/plus";
import Trash2 from "lucide-solid/icons/trash-2";
import Save from "lucide-solid/icons/save";
import {
	createEffect,
	createMemo,
	createResource,
	createSignal,
	For,
	Show,
} from "solid-js";
import { showToast } from "solid-notifications";
import { listPermissions, type PermissionItem } from "@/api/permission";
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
import { hasPermission } from "@/utils/permissions";

const SYSTEM_ROLES = new Set(["admin", "user", "ban"]);

export const Route = createFileRoute("/(app)/manage/roles/")({
	beforeLoad: () => {
		if (!hasPermission("role:read")) {
			throw redirect({ to: "/" });
		}
	},
	component: RolesPage,
	staticData: {
		title: "角色权限",
		icon: "icon-manage",
	},
});

function roleLabel(name: string) {
	if (name === "admin") return "管理员";
	if (name === "ban") return "封禁";
	if (name === "user") return "普通用户";
	return name;
}

function RolesPage() {
	const canManage = () => hasPermission("role:manage");
	const [roles, { refetch: refetchRoles }] = createResource(listRoles);
	const [permissions] = createResource(listPermissions);

	const [selectedRole, setSelectedRole] = createSignal("");
	const [selectedCodes, setSelectedCodes] = createSignal<Set<string>>(
		new Set(),
	);
	const [loadingPerms, setLoadingPerms] = createSignal(false);
	const [saving, setSaving] = createSignal(false);
	const [newRoleName, setNewRoleName] = createSignal("");
	const [creating, setCreating] = createSignal(false);
	const [deletingRole, setDeletingRole] = createSignal<string | null>(null);

	createEffect(() => {
		const list = roles();
		if (!list?.length) return;
		if (!list.some((r) => r.name === selectedRole())) {
			setSelectedRole(list[0].name);
		}
	});

	createEffect(() => {
		const role = selectedRole();
		if (!role) return;
		setLoadingPerms(true);
		getRolePermissions(role)
			.then((data) => setSelectedCodes(new Set<string>(data.permissions)))
			.catch(() => setSelectedCodes(new Set<string>()))
			.finally(() => setLoadingPerms(false));
	});

	const permissionList = createMemo(
		() => permissions() ?? ([] as PermissionItem[]),
	);

	const toggle = (code: string, checked: boolean) => {
		setSelectedCodes((prev) => {
			const next = new Set(prev);
			if (checked) next.add(code);
			else next.delete(code);
			return next;
		});
	};

	const handleSave = async () => {
		const role = selectedRole();
		if (!role) return;
		if (!canManage()) {
			showToast("无角色权限管理权限", { type: "error" });
			return;
		}
		setSaving(true);
		try {
			await syncRolePermissions(role, Array.from(selectedCodes()));
			showToast("角色权限已保存", { type: "success" });
		} catch {
		} finally {
			setSaving(false);
		}
	};

	const handleCreate = async () => {
		const name = newRoleName().trim();
		if (!name) {
			showToast("请输入角色名称", { type: "warning" });
			return;
		}
		if (!canManage()) {
			showToast("无角色管理权限", { type: "error" });
			return;
		}
		setCreating(true);
		try {
			await createRole(name);
			showToast("角色已创建", { type: "success" });
			setNewRoleName("");
			await refetchRoles();
			setSelectedRole(name);
		} catch {
		} finally {
			setCreating(false);
		}
	};

	const handleDelete = async (role: Role) => {
		if (!canManage()) {
			showToast("无角色管理权限", { type: "error" });
			return;
		}
		if (!confirm(`确认删除角色「${roleLabel(role.name)}」？`)) return;
		setDeletingRole(role.name);
		try {
			await deleteRole(role.id);
			showToast("角色已删除", { type: "success" });
			await refetchRoles();
		} catch {
		} finally {
			setDeletingRole(null);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<ShieldCheck size={18} />}
				title="角色权限"
				description="管理平台角色及其权限码"
			/>

			<Show
				when={!roles.loading}
				fallback={<ManageLoading label="正在加载角色..." />}
			>
				<div class="grid min-w-0 gap-4 md:grid-cols-[240px_minmax(0,1fr)]">
					<ManageSection
						title="角色"
						actions={
							<Show when={canManage()}>
								<div class="flex items-center gap-1.5">
									<input
										type="text"
										class="input input-bordered input-xs w-28"
										placeholder="新角色名"
										value={newRoleName()}
										onInput={(e) => setNewRoleName(e.currentTarget.value)}
										onKeyDown={(e) => {
											if (e.key === "Enter") void handleCreate();
										}}
									/>
									<button
										type="button"
										class="btn btn-primary btn-xs"
										disabled={creating()}
										onClick={() => void handleCreate()}
									>
										<Plus size={13} />
									</button>
								</div>
							</Show>
						}
					>
						<div class="flex flex-col gap-1">
							<For each={roles() ?? []}>
								{(role) => (
									<div class="flex items-center gap-1">
										<button
											type="button"
											class={`btn btn-ghost btn-sm flex-1 justify-start ${selectedRole() === role.name ? "btn-active" : ""}`}
											onClick={() => setSelectedRole(role.name)}
										>
											<span class="truncate">{roleLabel(role.name)}</span>
											<Show when={SYSTEM_ROLES.has(role.name)}>
												<span class="badge badge-ghost badge-xs">系统</span>
											</Show>
										</button>
										<Show when={canManage() && !SYSTEM_ROLES.has(role.name)}>
											<button
												type="button"
												class="btn btn-ghost btn-square btn-xs text-error"
												disabled={deletingRole() === role.name}
												onClick={() => void handleDelete(role)}
												aria-label={`删除角色 ${roleLabel(role.name)}`}
											>
												<Trash2 size={13} />
											</button>
										</Show>
									</div>
								)}
							</For>
						</div>
					</ManageSection>

					<ManageSection
						title={`权限 — ${roleLabel(selectedRole())}`}
						actions={
							<Show when={canManage()}>
								<button
									type="button"
									class="btn btn-primary btn-xs"
									disabled={saving() || loadingPerms()}
									onClick={() => void handleSave()}
								>
									<Save size={13} />
									保存
								</button>
							</Show>
						}
					>
						<Show
							when={!loadingPerms()}
							fallback={<ManageLoading label="正在加载权限..." />}
						>
							<div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
								<For each={permissionList()}>
									{(perm) => (
										<label class="flex cursor-pointer items-center gap-2 rounded-lg border border-base-300/70 px-3 py-2 text-sm hover:bg-base-200/40">
											<input
												type="checkbox"
												class="checkbox checkbox-xs"
												checked={selectedCodes().has(perm.code)}
												onInput={(e) =>
													toggle(perm.code, e.currentTarget.checked)
												}
												disabled={!canManage()}
											/>
											<span class="min-w-0">
												<span class="block truncate font-medium">
													{perm.name}
												</span>
												<span class="block truncate text-[11px] text-base-content/50">
													{perm.code}
												</span>
											</span>
										</label>
									)}
								</For>
							</div>
						</Show>
					</ManageSection>
				</div>
			</Show>
		</ManagePage>
	);
}
