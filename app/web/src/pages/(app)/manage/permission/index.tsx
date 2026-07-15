import { createFileRoute, redirect } from "@tanstack/solid-router";
import Save from "lucide-solid/icons/save";
import ShieldCheck from "lucide-solid/icons/shield-check";
import {
	createEffect,
	createMemo,
	createResource,
	createSignal,
	Show,
} from "solid-js";
import { showToast } from "solid-notifications";
import {
	createRole,
	deleteRole,
	getRolePermissions,
	listPermissions,
	listRoles,
	type PermissionItem,
	type RolePermissionsData,
	syncRolePermissions,
	updateRole,
} from "@/api/permission";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import { hasPermission } from "@/utils/permissions";
import PermissionGrid from "./components/PermissionGrid";
import RoleList from "./components/RoleList";
import { DOMAIN_LABELS, getDomain, isDefaultRole } from "./components/utils";

export const Route = createFileRoute("/(app)/manage/permission/")({
	beforeLoad: () => {
		if (!hasPermission("role:manage")) {
			throw redirect({ to: "/" });
		}
	},
	component: PermissionPage,
	staticData: {
		title: "权限",
		icon: "icon-manage",
	},
});

function PermissionPage() {
	const [selectedRole, setSelectedRole] = createSignal("");
	const [selectedCodes, setSelectedCodes] = createSignal<Set<string>>(
		new Set(),
	);
	const [saving, setSaving] = createSignal(false);
	const [newRoleName, setNewRoleName] = createSignal("");
	const [creating, setCreating] = createSignal(false);
	const [deleting, setDeleting] = createSignal(false);
	const [editingRoleId, setEditingRoleId] = createSignal<number | null>(null);
	const [editingName, setEditingName] = createSignal("");
	const [renaming, setRenaming] = createSignal(false);

	const [roles, { refetch: refetchRoles }] = createResource(listRoles);
	const [permissions] = createResource(listPermissions);

	const [rolePermCache, setRolePermCache] = createSignal<
		Map<string, RolePermissionsData>
	>(new Map());
	const [rolePermLoading, setRolePermLoading] = createSignal(false);

	createEffect(() => {
		const role = selectedRole();
		if (!role) return;

		const cached = rolePermCache().get(role);
		if (cached) {
			setSelectedCodes(new Set(cached.permissions));
			return;
		}

		setRolePermLoading(true);
		getRolePermissions(role)
			.then((data) => {
				setRolePermCache((prev) => {
					const next = new Map(prev);
					next.set(role, data);
					return next;
				});
				setSelectedCodes(new Set(data.permissions));
			})
			.catch(() => {
				setSelectedCodes(new Set<string>());
			})
			.finally(() => {
				setRolePermLoading(false);
			});
	});

	createEffect(() => {
		const firstRole = roles()?.[0]?.name;
		if (!selectedRole() && firstRole) setSelectedRole(firstRole);
	});

	const groupedPermissions = createMemo(() => {
		const groups = new Map<string, PermissionItem[]>();
		for (const permission of permissions() || []) {
			const domain = getDomain(permission);
			groups.set(domain, [...(groups.get(domain) || []), permission]);
		}
		return Array.from(groups.entries()).map(([domain, items]) => ({
			domain,
			label: DOMAIN_LABELS[domain] || domain,
			items,
		}));
	});

	const isDirty = createMemo(() => {
		const role = selectedRole();
		const original = new Set(rolePermCache().get(role)?.permissions || []);
		const current = selectedCodes();
		if (original.size !== current.size) return true;
		for (const code of current) {
			if (!original.has(code)) return true;
		}
		return false;
	});

	const loadError = createMemo(() => roles.error || permissions.error);

	const togglePermission = (code: string, checked: boolean) => {
		setSelectedCodes((current) => {
			const next = new Set(current);
			if (checked) {
				next.add(code);
			} else {
				next.delete(code);
			}
			return next;
		});
	};

	const startRename = (role: { id: number; name: string }) => {
		setEditingRoleId(role.id);
		setEditingName(role.name);
	};

	const cancelRename = () => {
		setEditingRoleId(null);
		setEditingName("");
	};

	const confirmRename = async () => {
		const id = editingRoleId();
		const newName = editingName().trim();
		if (!id || !newName) return;

		const currentRole = roles()?.find((r) => r.id === id);
		if (!currentRole) return;
		if (newName === currentRole.name) {
			cancelRename();
			return;
		}

		setRenaming(true);
		try {
			const oldName = currentRole.name;
			await updateRole(id, newName);
			await refetchRoles();

			setRolePermCache((prev) => {
				const next = new Map(prev);
				const cached = next.get(oldName);
				if (cached) {
					next.delete(oldName);
					next.set(newName, { ...cached, role: newName });
				}
				return next;
			});

			setSelectedRole(newName);
			showToast("角色已重命名", { type: "success" });
		} catch (error) {
			showToast(error instanceof Error ? error.message : "重命名失败", {
				type: "error",
			});
		} finally {
			setRenaming(false);
			cancelRename();
		}
	};

	const handleCreateRole = async () => {
		const name = newRoleName().trim();
		if (!name) return;
		setCreating(true);
		try {
			await createRole(name);
			setNewRoleName("");
			await refetchRoles();
			setSelectedRole(name);
			setRolePermCache((prev) => {
				const next = new Map(prev);
				next.set(name, { role: name, permissions: [] });
				return next;
			});
			setSelectedCodes(new Set<string>());
			showToast("角色已创建", { type: "success" });
		} catch (error) {
			showToast(error instanceof Error ? error.message : "创建失败", {
				type: "error",
			});
		} finally {
			setCreating(false);
		}
	};

	const handleDeleteRole = async (roleItem: { id: number; name: string }) => {
		if (isDefaultRole(roleItem.name)) return;
		setDeleting(true);
		try {
			await deleteRole(roleItem.id);
			setRolePermCache((prev) => {
				const next = new Map(prev);
				next.delete(roleItem.name);
				return next;
			});
			setSelectedRole("");
			await refetchRoles();
			showToast("角色已删除", { type: "success" });
		} catch (error) {
			showToast(error instanceof Error ? error.message : "删除失败", {
				type: "error",
			});
		} finally {
			setDeleting(false);
		}
	};

	const savePermissions = async () => {
		const role = selectedRole();
		if (!role) return;
		setSaving(true);
		try {
			const data = await syncRolePermissions(role, Array.from(selectedCodes()));
			setRolePermCache((prev) => {
				const next = new Map(prev);
				next.set(role, data);
				return next;
			});
			showToast("权限已保存", { type: "success" });
		} catch (error) {
			showToast(error instanceof Error ? error.message : "保存失败", {
				type: "error",
			});
		} finally {
			setSaving(false);
		}
	};

	return (
		<ManagePage class="h-full min-h-0 flex-1 overflow-hidden">
			<ManageHeader
				icon={<ShieldCheck size={18} />}
				title="权限"
				description="管理角色与权限分配"
				actions={
					<button
						type="button"
						class="btn btn-sm gap-2 border border-base-300 bg-base-100 text-base-content shadow-none hover:bg-base-200"
						disabled={!selectedRole() || saving() || !isDirty()}
						onClick={savePermissions}
						title="保存权限"
					>
						<Save size={16} />
						保存
					</button>
				}
			/>

			<Show when={loadError()}>
				<div class="alert alert-error py-2 text-sm">
					<span>{String(loadError()?.message || loadError())}</span>
				</div>
			</Show>

			<ManageSection
				class="min-h-0 flex-1"
				bodyClass="relative min-h-0 flex-1 overflow-hidden p-0 md:p-0"
				padded={false}
			>
				<div class="absolute inset-0 grid min-h-0 grid-cols-[12rem_minmax(0,1fr)] gap-0 max-md:grid-cols-1 max-md:grid-rows-[auto_minmax(0,1fr)]">
					<div class="min-h-0 overflow-y-auto overflow-x-hidden border-r border-base-300/70 p-3 max-md:border-r-0 max-md:border-b">
						<RoleList
							selectedRole={selectedRole()}
							setSelectedRole={setSelectedRole}
							roles={roles()}
							rolesLoading={!!roles.loading}
							renaming={renaming()}
							deleting={deleting()}
							editingRoleId={editingRoleId()}
							editingName={editingName()}
							setEditingRoleId={setEditingRoleId}
							setEditingName={setEditingName}
							newRoleName={newRoleName()}
							setNewRoleName={setNewRoleName}
							creating={creating()}
							onStartRename={startRename}
							onCancelRename={cancelRename}
							onConfirmRename={confirmRename}
							onCreateRole={handleCreateRole}
							onDeleteRole={handleDeleteRole}
						/>
					</div>
					<div class="min-h-0 overflow-y-auto overflow-x-hidden p-4">
						<PermissionGrid
							groupedPermissions={groupedPermissions()}
							selectedCodes={selectedCodes()}
							loading={rolePermLoading()}
							onToggle={togglePermission}
						/>
					</div>
				</div>
			</ManageSection>
		</ManagePage>
	);
}
