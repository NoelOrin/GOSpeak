import { createSignal, For, Show } from "solid-js";
import Plus from "lucide-solid/icons/plus";
import Save from "lucide-solid/icons/save";
import Trash2 from "lucide-solid/icons/trash-2";
import type { DomainRole } from "@/api/domain";

export default function DomainRolePanel(props: {
	roles: DomainRole[];
	assignable: string[];
	loading: boolean;
	saving: boolean;
	error: string;
	onCreate: (name: string, permissions: string[]) => Promise<void>;
	onUpdate: (roleName: string, permissions: string[]) => Promise<void>;
	onDelete: (roleName: string) => Promise<void>;
}) {
	const [selected, setSelected] = createSignal<string>("");
	const [selectedCodes, setSelectedCodes] = createSignal<Set<string>>(new Set());
	const [newRoleName, setNewRoleName] = createSignal("");

	const selectRole = (role: DomainRole) => {
		setSelected(role.name);
		setSelectedCodes(new Set(role.permissions));
	};

	const toggle = (code: string, checked: boolean) => {
		const next = new Set(selectedCodes());
		if (checked) next.add(code);
		else next.delete(code);
		setSelectedCodes(next);
	};

	const save = async () => {
		const name = selected();
		if (!name) return;
		await props.onUpdate(name, Array.from(selectedCodes()));
	};

	return (
		<div class="min-w-0">
			<Show when={props.error}>
				<div role="alert" class="alert alert-error mb-3 text-sm">
					<span>{props.error}</span>
				</div>
			</Show>
			<div class="grid min-w-0 gap-4 md:grid-cols-[220px_minmax(0,1fr)]">
				<div class="flex flex-col gap-1">
					<For each={props.roles}>
						{(role) => (
							<button
								type="button"
								class={`btn btn-ghost btn-sm justify-start ${selected() === role.name ? "btn-active" : ""}`}
								onClick={() => selectRole(role)}
							>
								<span class="truncate">{role.name}</span>
								{role.is_system ? (
									<span class="badge badge-ghost badge-xs">系统</span>
								) : null}
							</button>
						)}
					</For>
					<div class="mt-3 flex gap-2">
						<input
							class="input input-bordered input-sm min-w-0 flex-1"
							placeholder="新角色名"
							value={newRoleName()}
							onInput={(e) => setNewRoleName(e.currentTarget.value)}
						/>
						<button
							type="button"
							class="btn btn-primary btn-sm"
							disabled={!newRoleName().trim() || props.saving}
							onClick={() => {
								void props.onCreate(newRoleName().trim(), Array.from(selectedCodes()));
								setNewRoleName("");
							}}
						>
							<Plus size={14} />
							创建
						</button>
					</div>
				</div>
				<div class="min-w-0">
					<Show when={selected()} fallback={<p class="text-sm text-base-content/50">选择角色进行权限配置</p>}>
						<div class="mb-3 flex flex-wrap items-center justify-between gap-2">
							<h3 class="font-semibold">{selected()}</h3>
							<div class="flex gap-2">
								<Show when={!props.roles.find((r) => r.name === selected())?.is_system}>
									<button
										type="button"
										class="btn btn-outline btn-error btn-sm"
										disabled={props.saving}
										onClick={() => void props.onDelete(selected())}
									>
										<Trash2 size={14} />
										删除
									</button>
								</Show>
								<button
									type="button"
									class="btn btn-primary btn-sm"
									disabled={props.saving || selected() === "owner"}
									onClick={() => void save()}
								>
									<Save size={14} />
									保存权限
								</button>
							</div>
						</div>
						<div class="grid grid-cols-2 gap-2 xl:grid-cols-3 max-md:grid-cols-1">
							<For each={props.assignable}>
								{(code) => (
									<label class="flex cursor-pointer items-start gap-2 border border-base-300 px-3 py-2 text-sm">
										<input
											type="checkbox"
											class="checkbox checkbox-sm mt-0.5"
											checked={selectedCodes().has(code)}
											disabled={selected() === "owner"}
											onChange={(e) => toggle(code, e.currentTarget.checked)}
										/>
										<span class="break-all font-mono text-xs">{code}</span>
									</label>
								)}
							</For>
						</div>
					</Show>
				</div>
			</div>
		</div>
	);
}
