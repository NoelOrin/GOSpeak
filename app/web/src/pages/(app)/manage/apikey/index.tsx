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
import ConfirmModal from "@/components/common/ConfirmModal";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
	ManageTag,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import { hasPermission } from "@/utils/permissions";

const EXPIRY_OPTIONS = [
	{ value: "24h", label: "1 天" },
	{ value: "168h", label: "7 天" },
	{ value: "720h", label: "30 天" },
	{ value: "", label: "永不过期" },
];

const PERMANENT_YEAR = 2125;

export const Route = createFileRoute("/(app)/manage/apikey/")({
	beforeLoad: () => {
		if (!hasPermission("bot:manage")) {
			throw redirect({ to: "/" });
		}
	},
	component: ApiKeyPage,
	staticData: {
		title: "BOT 密钥",
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
	const [revokeTarget, setRevokeTarget] = createSignal<BotAPIKey | null>(null);
	let revokeDialogRef!: HTMLDialogElement;

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
			showToast("存在不允许授予 BOT 的权限", { type: "error" });
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

	const openRevokeModal = (key: BotAPIKey) => {
		setRevokeTarget(key);
		queueMicrotask(() => {
			revokeDialogRef?.showModal();
		});
	};

	const closeRevokeModal = () => {
		revokeDialogRef?.close();
		setRevokeTarget(null);
	};

	const handleRevoke = async () => {
		const key = revokeTarget();
		if (!key) return;
		setRevokingUuid(key.uuid);
		try {
			const res = await revokeBotKey(key.uuid);
			if (res.code !== 0) {
				showToast(res.msg || "吊销失败", { type: "error" });
				return;
			}
			showToast("密钥已吊销", { type: "success" });
			closeRevokeModal();
			refetch();
		} catch (e: any) {
			showToast(e?.message || "吊销失败", { type: "error" });
		} finally {
			setRevokingUuid(null);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<KeyRound size={18} />}
				title="BOT 密钥"
				description="创建并管理机器人访问密钥"
			/>

			<ManageSection title="生成新密钥" description="选择权限与过期时间">
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
								{(perm) => {
									const checked = () =>
										selectedPermissions().includes(perm.code);
									return (
										<label
											class="flex min-h-20 cursor-pointer items-start gap-3 rounded-xl border p-3 transition-colors"
											classList={{
												"border-base-content/25 bg-base-200/50": checked(),
												"border-base-300 hover:border-base-content/15 hover:bg-base-200/30":
													!checked(),
											}}
										>
											<input
												type="checkbox"
												class="checkbox checkbox-sm mt-1"
												checked={checked()}
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
									);
								}}
							</For>
						</div>
					</Show>
				</fieldset>

				<div class="mt-2 flex flex-wrap items-end justify-between gap-3">
					<fieldset class="fieldset min-w-52 flex-1">
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
					<button
						type="button"
						class="btn btn-sm border border-base-300 bg-base-100 text-base-content shadow-none hover:bg-base-200"
						classList={{ "btn-disabled": creating() }}
						onClick={handleCreate}
					>
						<Show when={creating()} fallback="生成密钥">
							<span class="loading loading-spinner loading-xs" /> 生成中...
						</Show>
					</button>
				</div>

				<Show when={newPlainKey()}>
					<div class="mt-3 rounded-xl border border-base-300 bg-base-200/30 px-4 py-3 text-sm">
						<div class="flex items-start gap-2">
							<KeyRound
								size={16}
								class="mt-0.5 shrink-0 text-base-content/50"
							/>
							<div class="min-w-0 flex-1 break-all">
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
					</div>
				</Show>
			</ManageSection>

			<ManageSection title="已创建的密钥" padded={false}>
				<Show
					when={!keysData.loading}
					fallback={<div class="loading loading-spinner loading-sm m-4" />}
				>
					<Show
						when={(keysData()?.data?.length ?? 0) > 0}
						fallback={
							<div class="m-4 rounded-xl border border-dashed border-base-300 bg-base-200/20 py-10 text-center text-sm text-base-content/55">
								暂无 BOT 密钥
							</div>
						}
					>
						<div class="overflow-x-auto">
							<table class="table table-sm">
								<thead>
									<tr class={manageTableHeadClass}>
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
											<tr class={manageTableRowClass}>
												<td class="font-medium">{key.name}</td>
												<td>
													<div class="flex flex-wrap gap-1">
														<For each={key.permissions}>
															{(perm) => (
																<ManageTag>{permissionLabel(perm)}</ManageTag>
															)}
														</For>
													</div>
												</td>
												<td class="text-xs text-base-content/75">
													{formatExpiry(key.expires_at)}
												</td>
												<td>
													<ManageTag>
														{key.revoked ? "已吊销" : "有效"}
													</ManageTag>
												</td>
												<td>
													<Show when={!key.revoked}>
														<button
															type="button"
															class="btn btn-ghost btn-xs text-base-content/60"
															disabled={revokingUuid() === key.uuid}
															onClick={() => openRevokeModal(key)}
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
			</ManageSection>

			<ConfirmModal
				open={!!revokeTarget()}
				title="吊销密钥"
				message={
					<span>确认吊销密钥「{revokeTarget()?.name}」？此操作不可撤销。</span>
				}
				confirmText="吊销"
				confirmClass="btn btn-error"
				loading={!!revokingUuid()}
				dialogRef={(el) => {
					revokeDialogRef = el;
				}}
				onClose={closeRevokeModal}
				onConfirm={handleRevoke}
			/>
		</ManagePage>
	);
}
