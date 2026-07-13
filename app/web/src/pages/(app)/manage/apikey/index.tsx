import { createFileRoute, redirect } from "@tanstack/solid-router";
import Copy from "lucide-solid/icons/copy";
import KeyRound from "lucide-solid/icons/key-round";
import Trash2 from "lucide-solid/icons/trash-2";
import { createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	BOT_ALLOWED_PERMISSION_CODES,
	type BotAPIKey,
	createBotKey,
	listBotKeys,
	revokeBotKey,
} from "@/api/apikey";
import { listPermissions, type PermissionItem } from "@/api/permission";
import userStore from "@/stores/userStore";

const EXPIRY_OPTIONS = [
	{ value: "24h", label: "1 天" },
	{ value: "168h", label: "7 天" },
	{ value: "720h", label: "30 天" },
	{ value: "", label: "永不过期" },
];

const PERMANENT_YEAR = 2125;

export const Route = createFileRoute("/(app)/manage/apikey/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
	},
	component: ApiKeyPage,
	staticData: {
		title: "Bot 密钥",
		icon: "icon-manage",
	},
});

function ApiKeyPage() {
	const [keysData, { refetch }] = createResource(() => listBotKeys());
	const [permissionsData] = createResource(() => listPermissions());

	const botPermissions = () =>
		(permissionsData() ?? []).filter((p: PermissionItem) =>
			BOT_ALLOWED_PERMISSION_CODES.includes(p.code),
		);

	const [name, setName] = createSignal("");
	const [selectedPermissions, setSelectedPermissions] = createSignal<string[]>(
		[],
	);
	const [expiry, setExpiry] = createSignal("720h");
	const [creating, setCreating] = createSignal(false);
	const [newPlainKey, setNewPlainKey] = createSignal("");
	const [revokingUuid, setRevokingUuid] = createSignal<string | null>(null);

	const permissionLabel = (code: string) =>
		permissionsData()?.find((p: PermissionItem) => p.code === code)?.name ||
		code;

	const formatExpiry = (value: string) => {
		const d = new Date(value);
		if (Number.isNaN(d.getTime())) return "-";
		if (d.getFullYear() >= PERMANENT_YEAR) return "永不过期";
		return d.toLocaleString();
	};

	const togglePermission = (code: string) => {
		setSelectedPermissions((prev) =>
			prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code],
		);
	};

	const handleCreate = async () => {
		if (!name().trim()) {
			showToast("请填写密钥名称", { type: "warning" });
			return;
		}
		const selected = selectedPermissions();
		if (selected.length === 0) {
			showToast("请至少选择一个权限", { type: "warning" });
			return;
		}
		if (!selected.every((c) => BOT_ALLOWED_PERMISSION_CODES.includes(c))) {
			showToast("存在不允许授予 Bot 的权限", { type: "error" });
			return;
		}
		setCreating(true);
		try {
			const res = await createBotKey({
				name: name().trim(),
				permissions: selectedPermissions(),
				expires_in: expiry() || undefined,
			});
			if (res.code !== 0) {
				showToast(res.msg || "创建失败", { type: "error" });
				return;
			}
			showToast("BOT 密钥已创建，请妥善保存明文 Key", { type: "success" });
			setNewPlainKey(res.data?.token || "");
			setName("");
			setSelectedPermissions([]);
			setExpiry("720h");
			refetch();
		} catch (e: any) {
			showToast(e?.message || "创建失败", { type: "error" });
		} finally {
			setCreating(false);
		}
	};

	const copyKey = async (key: string) => {
		try {
			await navigator.clipboard.writeText(key);
			showToast("已复制到剪贴板", { type: "success" });
		} catch {
			showToast("复制失败", { type: "error" });
		}
	};

	const handleRevoke = async (key: BotAPIKey) => {
		if (!confirm(`确认吊销密钥「${key.name}」？此操作不可撤销。`)) return;
		setRevokingUuid(key.uuid);
		try {
			const res = await revokeBotKey(key.uuid);
			if (res.code !== 0) {
				showToast(res.msg || "吊销失败", { type: "error" });
				return;
			}
			showToast("密钥已吊销", { type: "success" });
			refetch();
		} catch (e: any) {
			showToast(e?.message || "吊销失败", { type: "error" });
		} finally {
			setRevokingUuid(null);
		}
	};

	return (
		<div class="flex h-full min-h-0 flex-col gap-4 p-4 overflow-auto">
			<div class="flex items-center gap-2">
				<KeyRound size={20} />
				<h3 class="font-bold text-lg">Bot API Key 管理</h3>
			</div>

			{/* 新建表单 */}
			<div class="card bg-base-200 shadow-sm">
				<div class="card-body gap-3">
					<h3 class="font-bold text-base">生成新密钥</h3>

					<fieldset class="fieldset">
						<legend class="fieldset-legend text-[14px]">名称</legend>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="如 night-record-bot"
							value={name()}
							onInput={(e) => setName(e.target.value)}
						/>
					</fieldset>

					<fieldset class="fieldset">
						<legend class="fieldset-legend text-[14px]">
							权限（直接授予 Bot，不依赖角色）
						</legend>
						<Show
							when={!(permissionsData() === undefined)}
							fallback={<div class="loading loading-spinner loading-sm" />}
						>
							<div class="grid grid-cols-2 gap-2 xl:grid-cols-3 max-md:grid-cols-1">
								<For each={botPermissions()}>
									{(perm) => (
										<label class="flex min-h-20 items-start gap-3 rounded-md border border-base-300 p-3 hover:bg-base-200 cursor-pointer">
											<input
												type="checkbox"
												class="checkbox checkbox-sm mt-1"
												checked={selectedPermissions().includes(perm.code)}
												onChange={() => togglePermission(perm.code)}
											/>
											<span class="min-w-0 flex-1">
												<span class="block truncate font-medium text-sm">
													{perm.name}
												</span>
												<span class="mt-1 block break-all font-mono text-base-content/50 text-xs">
													{perm.code}
												</span>
												<span class="mt-1 line-clamp-2 block text-base-content/60 text-xs">
													{perm.description}
												</span>
											</span>
										</label>
									)}
								</For>
							</div>
						</Show>
					</fieldset>

					<fieldset class="fieldset">
						<legend class="fieldset-legend text-[14px]">过期时间</legend>
						<select
							class="select select-bordered select-sm w-full max-w-xs"
							value={expiry()}
							onChange={(e) => setExpiry(e.target.value)}
						>
							<For each={EXPIRY_OPTIONS}>
								{(opt) => <option value={opt.value}>{opt.label}</option>}
							</For>
						</select>
					</fieldset>

					<div>
						<button
							type="button"
							class="btn btn-primary btn-sm"
							classList={{ "btn-disabled": creating() }}
							onClick={handleCreate}
						>
							<Show when={creating()} fallback="生成密钥">
								<span class="loading loading-spinner loading-xs" /> 生成中...
							</Show>
						</button>
					</div>

					<Show when={newPlainKey()}>
						<div class="alert alert-warning text-sm">
							<KeyRound size={16} />
							<div class="flex-1 break-all">
								<div class="font-medium">
									请立即保存此明文 Key（仅显示一次）：
								</div>
								<code class="font-mono text-xs">{newPlainKey()}</code>
							</div>
							<button
								type="button"
								class="btn btn-sm btn-ghost"
								onClick={() => copyKey(newPlainKey())}
							>
								<Copy size={14} /> 复制
							</button>
						</div>
					</Show>
				</div>
			</div>

			{/* 密钥列表 */}
			<div>
				<div class="mb-2 font-semibold text-sm">已创建的密钥</div>
				<Show
					when={!keysData.loading}
					fallback={<div class="loading loading-spinner loading-sm" />}
				>
					<Show
						when={(keysData()?.data?.length ?? 0) > 0}
						fallback={
							<div class="text-base-content/50 py-8 text-center text-sm">
								暂无 BOT 密钥
							</div>
						}
					>
						<div class="overflow-x-auto">
							<table class="table table-zebra table-sm">
								<thead>
									<tr>
										<th>名称</th>
										<th>权限</th>
										<th>过期时间</th>
										<th>状态</th>
										<th>操作</th>
									</tr>
								</thead>
								<tbody>
									<For each={keysData()?.data ?? []}>
										{(key) => (
											<tr>
												<td class="font-medium">{key.name}</td>
												<td>
													<div class="flex flex-wrap gap-1">
														<For each={key.permissions}>
															{(perm) => (
																<span class="badge badge-ghost badge-sm">
																	{permissionLabel(perm)}
																</span>
															)}
														</For>
													</div>
												</td>
												<td class="text-xs">{formatExpiry(key.expires_at)}</td>
												<td>
													<Show
														when={!key.revoked}
														fallback={
															<span class="badge badge-error badge-sm">
																已吊销
															</span>
														}
													>
														<span class="badge badge-success badge-sm">
															有效
														</span>
													</Show>
												</td>
												<td>
													<Show when={!key.revoked}>
														<button
															type="button"
															class="btn btn-ghost btn-xs text-error"
															disabled={revokingUuid() === key.uuid}
															onClick={() => handleRevoke(key)}
														>
															<Show
																when={revokingUuid() === key.uuid}
																fallback={<Trash2 size={14} />}
															>
																<span class="loading loading-spinner loading-xs" />
															</Show>
															吊销
														</button>
													</Show>
												</td>
											</tr>
										)}
									</For>
								</tbody>
							</table>
						</div>
					</Show>
				</Show>
			</div>
		</div>
	);
}
