import { createFileRoute, redirect } from "@tanstack/solid-router";
import userStore from "@/stores/userStore";
import Save from "lucide-solid/icons/save";
import ShieldCheck from "lucide-solid/icons/shield-check";
import ShieldX from "lucide-solid/icons/shield-x";
import Plus from "lucide-solid/icons/plus";
import Trash2 from "lucide-solid/icons/trash-2";
import Pencil from "lucide-solid/icons/pencil";
import Check from "lucide-solid/icons/check";
import X from "lucide-solid/icons/x";
import {
	createEffect,
	createMemo,
	createResource,
	createSignal,
	For,
	Show,
} from "solid-js";
import { showToast } from "solid-notifications";
import {
	createRole,
	deleteRole,
	updateRole,
	getRolePermissions,
	listPermissions,
	listRoles,
	type PermissionItem,
	type RoleItem,
	type RolePermissionsData,
	syncRolePermissions,
} from "@/api/permission";

export const Route = createFileRoute("/(app)/manage/permission/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
	},
	component: PermissionPage,
	staticData: {
		title: "权限",
		icon: "icon-manage",
	},
});

const DOMAIN_LABELS: Record<string, string> = {
  bot: "BOT",
  room: "房间",
  user: "用户",
  role: "角色",
  signal: "信令",
  sfu: "SFU",
};

const getDomain = (permission: PermissionItem) =>
	permission.code.split(":")[0] || "other";

const isDefaultRole = (name: string) =>
	name === "admin" || name === "user" || name === "ban";

// ─── RoleList ─────────────────────────────────────────────

function RoleList(props: {
	selectedRole: string;
	setSelectedRole: (name: string) => void;
	roles: RoleItem[] | undefined;
	rolesLoading: boolean;
	renaming: boolean;
	deleting: boolean;
	editingRoleId: number | null;
	editingName: string;
	setEditingRoleId: (id: number | null) => void;
	setEditingName: (name: string) => void;
	newRoleName: string;
	setNewRoleName: (name: string) => void;
	creating: boolean;
	onStartRename: (role: { id: number; name: string }) => void;
	onCancelRename: () => void;
	onConfirmRename: () => void;
	onCreateRole: () => void;
	onDeleteRole: (role: { id: number; name: string }) => void;
}) {
	return (
		<div class="min-h-0 overflow-auto border-base-300 border-r pr-3 max-md:border-r-0 max-md:border-b max-md:pb-3">
			<div class="flex flex-col gap-1">
				<For each={props.roles || []}>
					{(role) => (
						<div class="relative group">
							<Show
								when={props.editingRoleId === role.id}
								fallback={
									<button
										type="button"
										class="btn btn-ghost justify-start gap-2 truncate w-full"
										classList={{
											"btn-active": props.selectedRole === role.name,
										}}
										onClick={() => props.setSelectedRole(role.name)}
										title={role.name}
									>
										<ShieldCheck size={16} />
										<span class="truncate">{role.name}</span>
									</button>
								}
							>
								<div class="flex items-center gap-1 px-1">
									<input
										type="text"
										class="input input-xs input-bordered flex-1 min-w-0"
										value={props.editingName}
										onInput={(e) => props.setEditingName(e.currentTarget.value)}
										onKeyDown={(e) => {
											if (e.key === "Enter") props.onConfirmRename();
											if (e.key === "Escape") props.onCancelRename();
										}}
										disabled={props.renaming}
									/>
									<button
										type="button"
										class="btn btn-ghost btn-xs"
										disabled={props.renaming}
										onClick={props.onConfirmRename}
									>
										<Check size={14} />
									</button>
									<button
										type="button"
										class="btn btn-ghost btn-xs"
										disabled={props.renaming}
										onClick={props.onCancelRename}
									>
										<X size={14} />
									</button>
								</div>
							</Show>
							<Show when={props.editingRoleId !== role.id}>
								<div class="absolute right-1 top-1/2 -translate-y-1/2 flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
									<button
										type="button"
										class="btn btn-ghost btn-xs"
										disabled={props.renaming}
										onClick={(e) => {
											e.stopPropagation();
											props.onStartRename(role);
										}}
										title="重命名"
									>
										<Pencil size={13} />
									</button>
									<Show when={!isDefaultRole(role.name)}>
										<button
											type="button"
											class="btn btn-ghost btn-xs text-error"
											disabled={props.deleting}
											onClick={(e) => {
												e.stopPropagation();
												props.onDeleteRole(role);
											}}
											title="删除角色"
										>
											<Trash2 size={13} />
										</button>
									</Show>
								</div>
							</Show>
						</div>
					)}
				</For>
			</div>
			<div class="mt-2 flex gap-1">
				<input
					type="text"
					class="input input-sm input-bordered flex-1 min-w-0"
					placeholder="新角色名"
					value={props.newRoleName}
					onInput={(e) => props.setNewRoleName(e.currentTarget.value)}
					onKeyDown={(e) => {
						if (e.key === "Enter") props.onCreateRole();
					}}
				/>
				<button
					type="button"
					class="btn btn-sm btn-ghost"
					disabled={!props.newRoleName.trim() || props.creating}
					onClick={props.onCreateRole}
					title="创建角色"
				>
					<Plus size={16} />
				</button>
			</div>
		</div>
	);
}

// ─── PermissionCard ────────────────────────────────────────

function PermissionCard(props: {
	permission: PermissionItem;
	checked: boolean;
	onToggle: (code: string, checked: boolean) => void;
}) {
	return (
		<label class="flex min-h-20 items-start gap-3 rounded-md border border-base-300 p-3 hover:bg-base-200 cursor-pointer">
			<input
				type="checkbox"
				class="checkbox checkbox-sm mt-1"
				checked={props.checked}
				onChange={(e) =>
					props.onToggle(props.permission.code, e.currentTarget.checked)
				}
			/>
			<span class="min-w-0 flex-1">
				<span class="block truncate font-medium text-sm">
					{props.permission.name}
				</span>
				<span class="mt-1 block break-all font-mono text-base-content/50 text-xs">
					{props.permission.code}
				</span>
				<span class="mt-1 line-clamp-2 block text-base-content/60 text-xs">
					{props.permission.description}
				</span>
			</span>
		</label>
	);
}

// ─── PermissionGrid ───────────────────────────────────────

function PermissionGrid(props: {
	groupedPermissions: { domain: string; label: string; items: PermissionItem[] }[];
	selectedCodes: Set<string>;
	loading: boolean;
	onToggle: (code: string, checked: boolean) => void;
}) {
	return (
		<div class="min-h-0 overflow-auto relative">
			<Show
				when={props.groupedPermissions.length > 0}
				fallback={<div class="loading loading-spinner loading-md" />}
			>
				<Show when={props.loading}>
					<div class="absolute inset-x-0 top-0 flex justify-center py-2 z-10">
						<div class="loading loading-spinner loading-sm" />
					</div>
				</Show>
				<div class="flex flex-col gap-5">
					<For each={props.groupedPermissions}>
						{(group) => (
							<section>
								<div class="mb-2 flex items-center gap-2 text-base-content/70 text-sm">
									<ShieldX size={15} />
									<span>{group.label}</span>
								</div>
								<div class="grid grid-cols-2 gap-2 xl:grid-cols-3 max-md:grid-cols-1">
									<For each={group.items}>
										{(permission) => (
											<PermissionCard
												permission={permission}
												checked={props.selectedCodes.has(permission.code)}
												onToggle={props.onToggle}
											/>
										)}
									</For>
								</div>
							</section>
						)}
					</For>
				</div>
			</Show>
		</div>
	);
}

// ─── PermissionPage ────────────────────────────────────────

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

	const handleDeleteRole = async (roleItem: {
		id: number;
		name: string;
	}) => {
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
		<div class="flex h-full min-h-0 flex-col gap-4 p-4 overflow-hidden">
			<div class="flex items-center justify-between gap-3">
				<h3 class="font-bold text-lg">权限</h3>
				<button
					type="button"
					class="btn btn-primary btn-sm gap-2"
					disabled={!selectedRole() || saving() || !isDirty()}
					onClick={savePermissions}
					title="保存权限"
				>
					<Save size={16} />
					保存
				</button>
			</div>

			<Show when={loadError()}>
				<div class="alert alert-error py-2 text-sm">
					<span>{String(loadError()?.message || loadError())}</span>
				</div>
			</Show>

			<div class="grid min-h-0 flex-1 grid-cols-[12rem_1fr] gap-4 max-md:grid-cols-1">
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
				<PermissionGrid
					groupedPermissions={groupedPermissions()}
					selectedCodes={selectedCodes()}
					loading={rolePermLoading()}
					onToggle={togglePermission}
				/>
			</div>
		</div>
	);
}
