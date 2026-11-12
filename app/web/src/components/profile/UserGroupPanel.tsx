import { toast } from "@gospeak/components";
import { Check, Pencil, Plus, Trash2, Users, X } from "lucide-react";
import { useEffect, useState } from "react";
import {
	createUserGroup,
	deleteUserGroup,
	listUserGroups,
	renameUserGroup,
	type UserGroup,
} from "@/api/userGroup";
import ConfirmModal from "@/components/common/ConfirmModal";
import { toastActionError } from "@/utils/toast";

const inputClass =
	"border-input bg-card text-foreground focus-visible:ring-ring placeholder:text-faint-foreground flex h-9 w-full rounded-sm border px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2";

export default function UserGroupPanel() {
	const [groups, setGroups] = useState<UserGroup[]>([]);
	const [loading, setLoading] = useState(true);
	const [newName, setNewName] = useState("");
	const [creating, setCreating] = useState(false);
	const [editingId, setEditingId] = useState<number | null>(null);
	const [editingName, setEditingName] = useState("");
	const [deleteTarget, setDeleteTarget] = useState<UserGroup | null>(null);
	const [deleting, setDeleting] = useState(false);

	useEffect(() => {
		void (async () => {
			setLoading(true);
			try {
				setGroups(await listUserGroups());
			} catch (err) {
				toastActionError(err, "加载分组失败");
			} finally {
				setLoading(false);
			}
		})();
	}, []);

	async function handleCreate() {
		const name = newName.trim();
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
			toast.success(`分组「${name}」已创建`);
		} catch (err) {
			toastActionError(err, "创建分组失败");
		} finally {
			setCreating(false);
		}
	}

	function startEdit(group: UserGroup) {
		setEditingId(group.id);
		setEditingName(group.group_name);
	}

	async function handleRename(group: UserGroup) {
		const name = editingName.trim();
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
			toast.success("分组已重命名");
		} catch (err) {
			toastActionError(err, "重命名分组失败");
		}
	}

	async function handleDelete() {
		const target = deleteTarget;
		if (!target) return;
		setDeleting(true);
		try {
			await deleteUserGroup(target.id);
			setGroups((prev) => prev.filter((item) => item.id !== target.id));
			setDeleteTarget(null);
			toast.success("分组已删除");
		} catch (err) {
			toastActionError(err, "删除分组失败");
		} finally {
			setDeleting(false);
		}
	}

	const iconBtn =
		"hover:bg-muted inline-flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-sm transition-colors disabled:pointer-events-none disabled:opacity-50";

	return (
		<section className="border-border bg-card rounded-lg border p-5 shadow-sm">
			<div className="flex items-start gap-3">
				<div className="bg-muted text-muted-foreground flex size-10 shrink-0 items-center justify-center rounded-lg">
					<Users size={20} />
				</div>
				<div>
					<h2 className="text-lg font-semibold">用户分组</h2>
					<p className="text-muted-foreground mt-1 text-sm leading-6">
						用命名分组整理常用联系人，方便后续快速筛选和发起会话。
					</p>
				</div>
			</div>

			<div className="mt-5 flex gap-2">
				<input
					type="text"
					value={newName}
					maxLength={50}
					placeholder="新分组名称"
					className={`${inputClass} flex-1`}
					onChange={(e) => setNewName(e.currentTarget.value)}
					onKeyDown={(e) => {
						if (e.key === "Enter") void handleCreate();
					}}
				/>
				<button
					type="button"
					className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-9 shrink-0 cursor-pointer items-center gap-1.5 rounded-sm px-3 text-sm transition-opacity disabled:pointer-events-none disabled:opacity-50"
					disabled={!newName.trim() || creating}
					onClick={() => void handleCreate()}
				>
					<Plus size={16} />
					创建
				</button>
			</div>

			<div className="mt-5">
				{!loading && groups.length > 0 ? (
					<div className="space-y-2">
						{groups.map((group) => (
							<div
								key={group.id}
								className="border-border/70 flex items-center gap-2 rounded-lg border px-3 py-2.5"
							>
								{editingId === group.id ? (
									<>
										<input
											type="text"
											value={editingName}
											maxLength={50}
											className="border-input bg-card text-foreground focus-visible:ring-ring min-w-0 h-8 flex-1 rounded-sm border px-2 text-xs focus-visible:outline-none focus-visible:ring-2"
											onChange={(e) => setEditingName(e.currentTarget.value)}
											onKeyDown={(e) => {
												if (e.key === "Enter") void handleRename(group);
												if (e.key === "Escape") setEditingId(null);
											}}
										/>
										<button
											type="button"
											className={`${iconBtn} text-success`}
											title="保存"
											onClick={() => void handleRename(group)}
										>
											<Check size={14} />
										</button>
										<button
											type="button"
											className={iconBtn}
											title="取消"
											onClick={() => setEditingId(null)}
										>
											<X size={14} />
										</button>
									</>
								) : (
									<>
										<span className="min-w-0 flex-1 truncate text-sm font-medium">
											{group.group_name}
										</span>
										<button
											type="button"
											className={iconBtn}
											title="重命名"
											onClick={() => startEdit(group)}
										>
											<Pencil size={14} />
										</button>
									</>
								)}
								<button
									type="button"
									className={`${iconBtn} hover:text-destructive`}
									title="删除"
									onClick={() => setDeleteTarget(group)}
								>
									<Trash2 size={14} />
								</button>
							</div>
						))}
					</div>
				) : (
					<div className="bg-muted/70 text-faint-foreground rounded-lg px-4 py-8 text-center text-sm">
						{loading ? "正在加载分组..." : "还没有用户分组"}
					</div>
				)}
			</div>

			<ConfirmModal
				open={!!deleteTarget}
				title="删除用户分组"
				message={`确定删除分组“${deleteTarget?.group_name || ""}”吗？`}
				confirmText="删除"
				loading={deleting}
				onClose={() => setDeleteTarget(null)}
				onConfirm={handleDelete}
			/>
		</section>
	);
}
