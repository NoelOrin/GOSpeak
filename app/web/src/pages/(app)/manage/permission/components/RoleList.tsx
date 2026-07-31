import Check from "lucide-solid/icons/check";
import Pencil from "lucide-solid/icons/pencil";
import Plus from "lucide-solid/icons/plus";
import ShieldCheck from "lucide-solid/icons/shield-check";
import Trash2 from "lucide-solid/icons/trash-2";
import X from "lucide-solid/icons/x";
import { For, Show } from "solid-js";
import type { RoleItem } from "@/api/permission";
import { isDefaultRole } from "./utils";

export default function RoleList(props: {
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
		<div class="min-h-0">
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
										class="btn btn-ghost btn-xs h-11 min-h-11 w-11 min-w-11"
										disabled={props.renaming}
										onClick={props.onConfirmRename}
									>
										<Check size={14} />
									</button>
									<button
										type="button"
										class="btn btn-ghost btn-xs h-11 min-h-11 w-11 min-w-11"
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
										class="btn btn-ghost btn-xs h-11 min-h-11 w-11 min-w-11"
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
											class="btn btn-ghost btn-xs h-11 min-h-11 w-11 min-w-11 text-base-content/70"
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
