import Check from "lucide-solid/icons/check";
import Pencil from "lucide-solid/icons/pencil";
import Plus from "lucide-solid/icons/plus";
import Trash2 from "lucide-solid/icons/trash-2";
import Users from "lucide-solid/icons/users";
import X from "lucide-solid/icons/x";
import { createSignal, For, onMount, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	createUserGroup,
	deleteUserGroup,
	listUserGroups,
	renameUserGroup,
	type UserGroup,
} from "@/api/userGroup";
import ConfirmModal from "@/components/common/ConfirmModal";

export default function UserGroupPanel() {
	const [groups, setGroups] = createSignal<UserGroup[]>([]);
	const [loading, setLoading] = createSignal(true);
	const [newName, setNewName] = createSignal("");
	const [creating, setCreating] = createSignal(false);
	const [editingId, setEditingId] = createSignal<number | null>(null);
	const [editingName, setEditingName] = createSignal("");
	const [deleteTarget, setDeleteTarget] = createSignal<UserGroup | null>(null);
	const [deleting, setDeleting] = createSignal(false);
	let deleteDialogRef!: HTMLDialogElement;

	onMount(load);

	async function load() {
		setLoading(true);
		try {
			setGroups(await listUserGroups());
		} catch (err) {
			showToast(err instanceof Error ? err.message : "加载分组失败", {
				type: "error",
			});
		} finally {
			setLoading(false);
		}
	}

	async function handleCreate() {
		const name = newName().trim();
		if (!name) return;
		setCreating(true);
		try {
			const group = await createUserGroup(name);
			setGroups((prev) =>
				[...prev, group].sort((a, b) =>
					a.group_name.localeCompare(b.group_name),
				),
			);
			setNewName("");
		} catch (err) {
			showToast(err instanceof Error ? err.message : "创建分组失败", {
				type: "error",
			});
		} finally {
			setCreating(false);
		}
	}

	function startEdit(group: UserGroup) {
		setEditingId(group.id);
		setEditingName(group.group_name);
	}

	async function handleRename(group: UserGroup) {
		const name = editingName().trim();
		if (!name || name === group.group_name) {
			setEditingId(null);
			return;
		}
		try {
			await renameUserGroup(group.id, name);
			setGroups((prev) =>
				prev
					.map((item) =>
						item.id === group.id ? { ...item, group_name: name } : item,
					)
					.sort((a, b) => a.group_name.localeCompare(b.group_name)),
			);
			setEditingId(null);
		} catch (err) {
			showToast(err instanceof Error ? err.message : "重命名失败", {
				type: "error",
			});
		}
	}

	async function handleDelete() {
		const target = deleteTarget();
		if (!target) return;
		setDeleting(true);
		try {
			await deleteUserGroup(target.id);
			setGroups((prev) => prev.filter((item) => item.id !== target.id));
			deleteDialogRef.close();
			setDeleteTarget(null);
		} catch (err) {
			showToast(err instanceof Error ? err.message : "删除分组失败", {
				type: "error",
			});
		} finally {
			setDeleting(false);
		}
	}

	return (
		<section class="rounded-lg border border-base-300 bg-base-100 p-5 shadow-sm">
			<div class="flex items-start gap-3">
				<div class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-base-200 text-base-content/70">
					<Users size={20} />
				</div>
				<div>
					<h2 class="text-lg font-semibold">用户分组</h2>
					<p class="mt-1 text-sm leading-6 text-base-content/60">
						用命名分组整理常用联系人，方便后续快速筛选和发起会话。
					</p>
				</div>
			</div>

			<div class="mt-5 flex gap-2">
				<input
					type="text"
					value={newName()}
					maxLength={50}
					placeholder="新分组名称"
					class="input input-sm input-bordered flex-1"
					onInput={(e) => setNewName(e.currentTarget.value)}
					onKeyDown={(e) => {
						if (e.key === "Enter") void handleCreate();
					}}
				/>
				<button
					type="button"
					class="btn btn-primary btn-sm shrink-0"
					disabled={!newName().trim() || creating()}
					onClick={() => void handleCreate()}
				>
					{creating() ? (
						<span class="loading loading-spinner loading-xs" />
					) : (
						<Plus size={16} />
					)}
					创建
				</button>
			</div>

			<div class="mt-5">
				<Show
					when={!loading() && groups().length > 0}
					fallback={
						<div class="rounded-lg bg-base-200/70 px-4 py-8 text-center text-sm text-base-content/50">
							{loading() ? "正在加载分组..." : "还没有用户分组"}
						</div>
					}
				>
					<div class="space-y-2">
						<For each={groups()}>
							{(group) => (
								<div class="flex items-center gap-2 rounded-lg border border-base-300/70 px-3 py-2.5">
									<Show
										when={editingId() === group.id}
										fallback={
											<span class="min-w-0 flex-1 truncate text-sm font-medium">
												{group.group_name}
											</span>
										}
									>
										<input
											type="text"
											value={editingName()}
											maxLength={50}
											class="input input-xs input-bordered min-w-0 flex-1"
											onInput={(e) => setEditingName(e.currentTarget.value)}
											onKeyDown={(e) => {
												if (e.key === "Enter") void handleRename(group);
												if (e.key === "Escape") setEditingId(null);
											}}
										/>
									</Show>
									<Show
										when={editingId() === group.id}
										fallback={
											<button
												type="button"
												class="btn btn-ghost btn-xs btn-square"
												title="重命名"
												onClick={() => startEdit(group)}
											>
												<Pencil size={14} />
											</button>
										}
									>
										<button
											type="button"
											class="btn btn-ghost btn-xs btn-square text-success"
											title="保存"
											onClick={() => void handleRename(group)}
										>
											<Check size={14} />
										</button>
										<button
											type="button"
											class="btn btn-ghost btn-xs btn-square"
											title="取消"
											onClick={() => setEditingId(null)}
										>
											<X size={14} />
										</button>
									</Show>
									<button
										type="button"
										class="btn btn-ghost btn-xs btn-square hover:text-error"
										title="删除"
										onClick={() => {
											setDeleteTarget(group);
											requestAnimationFrame(() => deleteDialogRef?.showModal());
										}}
									>
										<Trash2 size={14} />
									</button>
								</div>
							)}
						</For>
					</div>
				</Show>
			</div>

			<ConfirmModal
				open={!!deleteTarget()}
				dialogRef={(el) => {
					deleteDialogRef = el;
				}}
				title="删除用户分组"
				message={`确定删除分组“${deleteTarget()?.group_name || ""}”吗？`}
				confirmText="删除"
				loading={deleting()}
				onClose={() => {
					if (!deleting()) {
						setDeleteTarget(null);
						deleteDialogRef.close();
					}
				}}
				onConfirm={handleDelete}
			/>
		</section>
	);
}
