import { createFileRoute, redirect } from "@tanstack/solid-router";
import Blocks from "lucide-solid/icons/blocks";
import LayoutGrid from "lucide-solid/icons/layout-grid";
import List from "lucide-solid/icons/list";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Search from "lucide-solid/icons/search";
import Settings2 from "lucide-solid/icons/settings-2";
import {
	createEffect,
	createMemo,
	createResource,
	createSignal,
	For,
	Show,
} from "solid-js";
import { showToast } from "solid-notifications";
import { listPlugins, type PluginInfo, updatePlugin } from "@/api/plugin";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import { hasPermission } from "@/utils/permissions";
import PluginSettingsModal from "./components/PluginSettingsModal";
import {
	kindLabel,
	type LLMProviderForm,
	PAGE_SIZE_OPTIONS,
	statusMeta,
	type ViewMode,
} from "./components/shared";

export const Route = createFileRoute("/(app)/manage/bot-plugins/")({
	beforeLoad: () => {
		if (!hasPermission("plugin:read") && !hasPermission("bot:manage")) {
			throw redirect({ to: "/" });
		}
	},
	component: BotPluginsPage,
	staticData: {
		title: "BOT 插件",
		icon: "icon-manage",
	},
});

function BotPluginsPage() {
	const canManage = () =>
		hasPermission("plugin:manage") || hasPermission("bot:manage");

	const [pluginsData, { refetch }] = createResource(() => listPlugins());
	const [query, setQuery] = createSignal("");
	const [statusFilter, setStatusFilter] = createSignal("all");
	const [kindFilter, setKindFilter] = createSignal("all");
	const [viewMode, setViewMode] = createSignal<ViewMode>("cards");
	const [page, setPage] = createSignal(1);
	const [pageSize, setPageSize] = createSignal<number>(12);

	const [settingsOpen, setSettingsOpen] = createSignal(false);
	const [editingPlugin, setEditingPlugin] = createSignal<PluginInfo | null>(
		null,
	);
	const [saving, setSaving] = createSignal(false);

	const [enabled, setEnabled] = createSignal(true);
	const [sideEnabled, setSideEnabled] = createSignal(false);
	const [sideAddr, setSideAddr] = createSignal("127.0.0.1:9200");
	const [defaultProvider, setDefaultProvider] = createSignal("");
	const [providers, setProviders] = createSignal<LLMProviderForm[]>([]);

	const hydrate = (info?: PluginInfo | null) => {
		if (!info) return;
		setEnabled(!!info.enabled);
		const cfg = info.config ?? {};
		const side = (cfg.side_server ?? {}) as {
			enabled?: boolean;
			addr?: string;
		};
		setSideEnabled(!!side.enabled);
		setSideAddr(side.addr || "127.0.0.1:9200");
		setDefaultProvider((cfg.default_provider as string) || "");
		const list = Array.isArray(cfg.llm_providers)
			? (cfg.llm_providers as any[])
			: [];
		setProviders(
			list.map((p) => ({
				name: String(p.name ?? ""),
				display_name: String(p.display_name ?? p.name ?? ""),
				protocol: String(p.protocol ?? "openai-compatible"),
				base_url: String(p.base_url ?? ""),
				api_key: "",
				model: String(p.model ?? ""),
				enabled: p.enabled !== false,
			})),
		);
	};

	const filteredPlugins = createMemo(() => {
		const q = query().trim().toLowerCase();
		const status = statusFilter();
		const kind = kindFilter();
		return (pluginsData() ?? []).filter((p) => {
			if (status !== "all" && p.status !== status) return false;
			if (kind !== "all" && p.kind !== kind) return false;
			if (!q) return true;
			const haystack = [
				p.name,
				p.display_name,
				p.author,
				p.desc,
				p.version,
				p.kind,
				p.status,
			]
				.filter(Boolean)
				.join(" ")
				.toLowerCase();
			return haystack.includes(q);
		});
	});

	const totalPages = createMemo(() =>
		Math.max(1, Math.ceil(filteredPlugins().length / pageSize())),
	);

	const pagedPlugins = createMemo(() => {
		const start = (page() - 1) * pageSize();
		return filteredPlugins().slice(start, start + pageSize());
	});

	const pageNumbers = createMemo(() => {
		const total = totalPages();
		const current = page();
		const windowSize = 5;
		let start = Math.max(1, current - Math.floor(windowSize / 2));
		const end = Math.min(total, start + windowSize - 1);
		start = Math.max(1, end - windowSize + 1);
		return Array.from({ length: end - start + 1 }, (_, i) => start + i);
	});

	createEffect(() => {
		query();
		statusFilter();
		kindFilter();
		pageSize();
		setPage(1);
	});

	createEffect(() => {
		const total = totalPages();
		if (page() > total) setPage(total);
	});

	const openSettings = (plugin: PluginInfo) => {
		setEditingPlugin(plugin);
		hydrate(plugin);
		setSettingsOpen(true);
	};

	const closeSettings = () => {
		setSettingsOpen(false);
		setEditingPlugin(null);
	};

	const handleSave = async () => {
		if (!canManage()) {
			showToast("无插件管理权限", { type: "error" });
			return;
		}
		const info = editingPlugin();
		if (!info) return;

		const existing = Array.isArray(info.config?.llm_providers)
			? (info.config?.llm_providers as any[])
			: [];
		const existingKeyMap = new Map(
			existing.map((p) => [String(p.name), String(p.api_key ?? "")]),
		);

		const llm_providers = providers().map((p) => {
			const name = p.name.trim();
			const api_key =
				p.api_key.trim() ||
				existingKeyMap.get(name) ||
				existingKeyMap.get(p.name) ||
				"";
			return {
				name,
				display_name: p.display_name.trim() || name,
				protocol: p.protocol,
				base_url: p.base_url.trim(),
				api_key,
				model: p.model.trim(),
				enabled: p.enabled,
			};
		});

		if (llm_providers.some((p) => !p.name)) {
			showToast("供应商 name 不能为空", { type: "warning" });
			return;
		}
		const names = new Set(llm_providers.map((p) => p.name));
		if (names.size !== llm_providers.length) {
			showToast("供应商 name 不能重复", { type: "warning" });
			return;
		}

		const config = {
			side_server: {
				enabled: sideEnabled(),
				addr: sideAddr().trim(),
			},
			default_provider: defaultProvider().trim(),
			llm_providers,
		};

		setSaving(true);
		try {
			const updated = await updatePlugin({
				name: info.name,
				enabled: enabled(),
				config,
				restart: true,
			});
			showToast("插件配置已保存", { type: "success" });
			setEditingPlugin(updated);
			hydrate(updated);
			await refetch();
			closeSettings();
		} catch {
		} finally {
			setSaving(false);
		}
	};

	const rangeText = createMemo(() => {
		const total = filteredPlugins().length;
		if (total === 0) return "共 0 个插件";
		const start = (page() - 1) * pageSize() + 1;
		const end = Math.min(page() * pageSize(), total);
		return `显示 ${start}-${end} / 共 ${total} 个`;
	});

	return (
		<ManagePage>
			<ManageHeader
				icon={<Blocks size={18} />}
				title="BOT 插件"
				description="浏览后端挂载的 BOT 插件，点击卡片进入二级设置"
				actions={
					<button
						type="button"
						class="btn btn-ghost btn-sm gap-1.5"
						onClick={() => refetch()}
					>
						<RefreshCcw size={14} />
						刷新
					</button>
				}
			/>

			<ManageSection
				title="插件目录"
				description="支持搜索、筛选、分页，以及卡片 / 列表切换"
				actions={
					<div class="flex items-center gap-1 rounded-xl border border-base-300/80 bg-base-100 p-1">
						<button
							type="button"
							class="btn btn-ghost btn-xs gap-1"
							classList={{ "btn-active": viewMode() === "cards" }}
							onClick={() => setViewMode("cards")}
						>
							<LayoutGrid size={14} />
							卡片
						</button>
						<button
							type="button"
							class="btn btn-ghost btn-xs gap-1"
							classList={{ "btn-active": viewMode() === "list" }}
							onClick={() => setViewMode("list")}
						>
							<List size={14} />
							列表
						</button>
					</div>
				}
			>
				<div class="mb-4 flex flex-nowrap items-center gap-2 overflow-x-auto">
					<label class="input input-bordered input-sm flex min-w-[14rem] flex-1 items-center gap-2">
						<Search size={14} class="shrink-0 text-base-content/40" />
						<input
							type="search"
							class="min-w-0 grow bg-transparent outline-none"
							placeholder="搜索名称、作者、描述..."
							value={query()}
							onInput={(e) => setQuery(e.currentTarget.value)}
						/>
					</label>
					<select
						class="select select-bordered select-sm w-28 shrink-0"
						value={statusFilter()}
						onChange={(e) => setStatusFilter(e.currentTarget.value)}
					>
						<option value="all">全部状态</option>
						<option value="running">运行中</option>
						<option value="stopped">已停止</option>
						<option value="starting">启动中</option>
						<option value="failed">失败</option>
						<option value="registered">已注册</option>
					</select>
					<select
						class="select select-bordered select-sm w-28 shrink-0"
						value={kindFilter()}
						onChange={(e) => setKindFilter(e.currentTarget.value)}
					>
						<option value="all">全部类型</option>
						<option value="builtin">内置</option>
						<option value="external">外部</option>
					</select>
					<select
						class="select select-bordered select-sm w-24 shrink-0"
						value={String(pageSize())}
						onChange={(e) => setPageSize(Number(e.currentTarget.value))}
					>
						<For each={[...PAGE_SIZE_OPTIONS]}>
							{(size) => <option value={size}>{size} / 页</option>}
						</For>
					</select>
				</div>

				<Show
					when={!(pluginsData.loading && !pluginsData())}
					fallback={
						<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
							<For each={[1, 2, 3, 4, 5, 6]}>
								{() => <div class="skeleton h-40 w-full rounded-2xl" />}
							</For>
						</div>
					}
				>
					<Show
						when={filteredPlugins().length > 0}
						fallback={
							<div class="rounded-2xl border border-dashed border-base-300 px-4 py-14 text-center">
								<div class="text-sm font-medium text-base-content/70">
									没有匹配的插件
								</div>
								<div class="mt-1 text-xs text-base-content/45">
									试试调整搜索词或筛选条件
								</div>
							</div>
						}
					>
						<Show
							when={viewMode() === "cards"}
							fallback={
								<div class="overflow-x-auto rounded-2xl border border-base-300/80">
									<table class="table table-sm">
										<thead>
											<tr class={manageTableHeadClass}>
												<th>插件</th>
												<th class="w-28">类型</th>
												<th class="w-28">状态</th>
												<th class="w-24">版本</th>
												<th class="w-28">启用</th>
												<th class="w-28">操作</th>
											</tr>
										</thead>
										<tbody>
											<For each={pagedPlugins()}>
												{(plugin) => {
													const meta = () => statusMeta(plugin.status);
													return (
														<tr class={manageTableRowClass}>
															<td>
																<div class="min-w-0">
																	<div class="truncate font-medium text-base-content">
																		{plugin.display_name || plugin.name}
																	</div>
																	<div class="mt-0.5 truncate font-mono text-[11px] text-base-content/45">
																		{plugin.name}
																	</div>
																	<div class="mt-1 line-clamp-1 text-xs text-base-content/50">
																		{plugin.desc || "无描述"}
																	</div>
																</div>
															</td>
															<td>
																<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2 py-0.5 text-[11px] font-medium text-base-content/70">
																	{kindLabel(plugin.kind)}
																</span>
															</td>
															<td>
																<span
																	class={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-medium ${meta().chip}`}
																>
																	<span
																		class={`size-1.5 rounded-full ${meta().dot}`}
																	/>
																	{meta().label}
																</span>
															</td>
															<td class="font-mono text-xs text-base-content/60">
																v{plugin.version || "-"}
															</td>
															<td>
																<span class="inline-flex items-center rounded-full border border-base-300 bg-base-100 px-2 py-0.5 text-[11px] font-medium text-base-content/65">
																	{plugin.enabled ? "已启用" : "已禁用"}
																</span>
															</td>
															<td>
																<button
																	type="button"
																	class="btn btn-ghost btn-xs gap-1"
																	onClick={() => openSettings(plugin)}
																>
																	<Settings2 size={13} />
																	设置
																</button>
															</td>
														</tr>
													);
												}}
											</For>
										</tbody>
									</table>
								</div>
							}
						>
							<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
								<For each={pagedPlugins()}>
									{(plugin) => {
										const meta = () => statusMeta(plugin.status);
										return (
											<article class="flex h-full flex-col rounded-2xl border border-base-300/80 bg-base-100 p-4 transition-colors hover:bg-base-200/20">
												<div class="flex items-start justify-between gap-3">
													<div class="min-w-0">
														<div class="truncate text-sm font-semibold text-base-content">
															{plugin.display_name || plugin.name}
														</div>
														<div class="mt-1 truncate font-mono text-[11px] text-base-content/45">
															{plugin.name}
														</div>
													</div>
													<span
														class={`inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-medium ${meta().chip}`}
													>
														<span
															class={`size-1.5 rounded-full ${meta().dot}`}
														/>
														{meta().label}
													</span>
												</div>

												<p class="mt-3 line-clamp-2 min-h-10 text-xs leading-5 text-base-content/55">
													{plugin.desc || "无描述"}
												</p>

												<div class="mt-3 flex flex-wrap items-center gap-1.5">
													<span class="rounded-md border border-base-300/80 bg-base-100 px-1.5 py-0.5 text-[11px] text-base-content/55">
														{kindLabel(plugin.kind)}
													</span>
													<span class="rounded-md border border-base-300/80 bg-base-100 px-1.5 py-0.5 text-[11px] text-base-content/55">
														v{plugin.version || "-"}
													</span>
													<span class="rounded-md border border-base-300/80 bg-base-100 px-1.5 py-0.5 text-[11px] text-base-content/55">
														{plugin.enabled ? "已启用" : "已禁用"}
													</span>
													<Show when={plugin.author}>
														<span class="rounded-md border border-base-300/80 bg-base-100 px-1.5 py-0.5 text-[11px] text-base-content/55">
															{plugin.author}
														</span>
													</Show>
												</div>

												<div class="mt-auto flex items-center justify-between gap-2 pt-4">
													<div class="truncate text-[11px] text-base-content/40">
														{(plugin.side_servers?.length ?? 0) > 0
															? `${plugin.side_servers?.length} 个 Side Server`
															: "无 Side Server"}
													</div>
													<button
														type="button"
														class="btn btn-outline btn-sm gap-1.5"
														onClick={() => openSettings(plugin)}
													>
														<Settings2 size={14} />
														设置
													</button>
												</div>
											</article>
										);
									}}
								</For>
							</div>
						</Show>
					</Show>
				</Show>

				<div class="mt-4 flex flex-col gap-3 border-t border-base-300/70 pt-4 sm:flex-row sm:items-center sm:justify-between">
					<div class="text-xs text-base-content/50">{rangeText()}</div>
					<div class="flex flex-wrap items-center gap-1.5">
						<button
							type="button"
							class="btn btn-ghost btn-xs"
							disabled={page() <= 1}
							onClick={() => setPage((p) => Math.max(1, p - 1))}
						>
							上一页
						</button>
						<For each={pageNumbers()}>
							{(n) => (
								<button
									type="button"
									class="btn btn-ghost btn-xs min-w-11"
									classList={{ "btn-active": page() === n }}
									onClick={() => setPage(n)}
								>
									{n}
								</button>
							)}
						</For>
						<button
							type="button"
							class="btn btn-ghost btn-xs"
							disabled={page() >= totalPages()}
							onClick={() => setPage((p) => Math.min(totalPages(), p + 1))}
						>
							下一页
						</button>
					</div>
				</div>
			</ManageSection>

			<PluginSettingsModal
				open={settingsOpen}
				plugin={editingPlugin}
				canManage={canManage}
				saving={saving}
				enabled={enabled}
				setEnabled={setEnabled}
				sideEnabled={sideEnabled}
				setSideEnabled={setSideEnabled}
				sideAddr={sideAddr}
				setSideAddr={setSideAddr}
				defaultProvider={defaultProvider}
				setDefaultProvider={setDefaultProvider}
				providers={providers}
				setProviders={setProviders}
				onClose={closeSettings}
				onSave={handleSave}
			/>
		</ManagePage>
	);
}
