import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import {
	createMemo,
	createResource,
	createSignal,
	For,
	onMount,
	Show,
} from "solid-js";
import {
	type Guild,
	joinGuild,
	listPublicGuilds,
	previewGuildInvite,
} from "@/api/guild";
import CreateGuildModal from "@/components/guild/CreateGuildModal";
import GuildIcon from "@/components/guild/GuildIcon";
import GuildInvitePreview from "@/components/guild/GuildInvitePreview";
import guildStore from "@/stores/guildStore";
import { extractGuildInviteCode } from "@/utils/guildInvite";

export const Route = createFileRoute("/(app)/discover/")({
	component: RouteComponent,
	staticData: {
		title: "发现服务器",
		icon: "compass",
	},
});

const PAGE_SIZE = 12;

function RouteComponent() {
	const navigate = useNavigate();
	const [keyword, setKeyword] = createSignal("");
	const [searchInput, setSearchInput] = createSignal("");
	const [page, setPage] = createSignal(1);
	const [inviteInput, setInviteInput] = createSignal("");

	const [previewOpen, setPreviewOpen] = createSignal(false);
	const [previewLoading, setPreviewLoading] = createSignal(false);
	const [previewGuild, setPreviewGuild] = createSignal<Guild | null>(null);
	const [previewCode, setPreviewCode] = createSignal("");
	const [previewError, setPreviewError] = createSignal("");
	const [joining, setJoining] = createSignal(false);

	const [createRef, setCreateRef] = createSignal<HTMLDialogElement>();

	const [publicGuilds, { refetch }] = createResource(
		() => ({ keyword: keyword(), page: page(), pageSize: PAGE_SIZE }),
		({ keyword, page, pageSize }) =>
			listPublicGuilds(page, pageSize, keyword || undefined),
	);

	const totalPages = createMemo(() =>
		Math.max(1, Math.ceil((publicGuilds()?.total ?? 0) / PAGE_SIZE)),
	);

	const isJoined = (uuid: string) =>
		guildStore.state.myGuildUUIDs.includes(uuid);

	const previewJoined = () => {
		const guild = previewGuild();
		return !!guild && isJoined(guild.uuid);
	};

	async function openPreview(code: string) {
		setPreviewOpen(true);
		setPreviewLoading(true);
		setPreviewError("");
		setPreviewGuild(null);
		setPreviewCode(code);
		try {
			setPreviewGuild(await previewGuildInvite(code));
		} catch (e: any) {
			setPreviewError(e?.response?.data?.msg || "邀请码无效");
		} finally {
			setPreviewLoading(false);
		}
	}

	function handleInviteSubmit(e: Event) {
		e.preventDefault();
		const code = extractGuildInviteCode(inviteInput());
		if (!code) {
			setPreviewOpen(true);
			setPreviewError("未识别到邀请码或邀请链接");
			return;
		}
		void openPreview(code);
	}

	function handleSearch(e: Event) {
		e.preventDefault();
		setPage(1);
		setKeyword(searchInput().trim());
	}

	async function joinPreview() {
		const guild = previewGuild();
		const code = previewCode();
		if (!guild || !code) return;
		if (joining()) return;
		if (isJoined(guild.uuid)) {
			setPreviewOpen(false);
			navigate({
				to: "/guild/$guildUUID",
				params: { guildUUID: guild.uuid },
			});
			return;
		}
		setJoining(true);
		setPreviewError("");
		try {
			const joined = await joinGuild(code);
			guildStore.addGuild(joined);
			guildStore.setCurrentGuild(joined.uuid);
			setPreviewOpen(false);
			navigate({
				to: "/guild/$guildUUID",
				params: { guildUUID: joined.uuid },
			});
		} catch (e: any) {
			setPreviewError(e?.response?.data?.msg || "加入失败");
		} finally {
			setJoining(false);
		}
	}

	onMount(() => {
		guildStore.loadMyGuilds();
		void (async () => {
			try {
				if (!navigator.clipboard?.readText) return;
				const text = await navigator.clipboard.readText();
				const code = extractGuildInviteCode(text);
				if (!code) return;
				setInviteInput(code);
				await openPreview(code);
			} catch {
				// 浏览器未授权读取剪贴板时静默处理，用户可手动输入。
			}
		})();
	});

	return (
		<div class="flex-1 min-w-0 h-full overflow-y-auto p-4 sm:p-6">
			<div class="max-w-5xl mx-auto">
				<div class="flex flex-wrap items-center justify-between gap-3 mb-4">
					<h1 class="text-2xl font-bold">发现服务器</h1>
					<button
						type="button"
						class="btn btn-primary"
						onClick={() => createRef()?.showModal()}
					>
						新建服务器
					</button>
				</div>

				<form class="flex gap-2 mb-3" onSubmit={handleSearch}>
					<input
						type="search"
						class="input input-bordered flex-1 min-w-0"
						value={searchInput()}
						onInput={(e) => setSearchInput(e.currentTarget.value)}
						placeholder="搜索服务器名称或描述"
					/>
					<button type="submit" class="btn btn-outline">
						搜索
					</button>
				</form>

				<form class="flex gap-2 mb-6" onSubmit={handleInviteSubmit}>
					<input
						type="text"
						class="input input-bordered flex-1 min-w-0"
						value={inviteInput()}
						onInput={(e) => setInviteInput(e.currentTarget.value)}
						placeholder="邀请码或邀请链接"
					/>
					<button type="submit" class="btn btn-outline">
						查看
					</button>
				</form>

				<Show when={publicGuilds.loading && !publicGuilds()}>
					<div class="flex justify-center py-10">
						<span class="loading loading-spinner loading-md" />
					</div>
				</Show>

				<Show when={!publicGuilds.loading && publicGuilds.error}>
					<div role="alert" class="alert alert-error flex-wrap justify-between">
						<span>服务器列表加载失败</span>
						<button
							type="button"
							class="btn btn-sm"
							onClick={() => void refetch()}
						>
							重试
						</button>
					</div>
				</Show>

				<Show when={publicGuilds.loading && publicGuilds()}>
					<div class="flex justify-center py-3">
						<span class="loading loading-spinner loading-sm" />
					</div>
				</Show>

				<Show
					when={!publicGuilds.loading && !publicGuilds.error && publicGuilds()}
				>
					<Show
						when={(publicGuilds()?.guilds?.length ?? 0) > 0}
						fallback={
							<div class="py-10 text-center text-base-content/50">
								暂无公开服务器
							</div>
						}
					>
						<div class="grid gap-3 md:grid-cols-2">
							<For each={publicGuilds()?.guilds ?? []}>
								{(guild) => (
									<div class="card bg-base-100 border border-base-300 p-4">
										<div class="flex items-start gap-3">
											<GuildIcon
												name={guild.name}
												iconUrl={guild.icon_url}
												class="shrink-0"
											/>
											<div class="min-w-0 flex-1">
												<div class="flex items-center justify-between gap-2">
													<span class="font-semibold truncate">
														{guild.name}
													</span>
													<span class="badge badge-ghost badge-sm shrink-0">
														公开
													</span>
												</div>
												<p class="mt-1 text-sm text-base-content/60 line-clamp-2">
													{guild.description || "暂无描述"}
												</p>
												<div class="mt-3">
													<Show
														when={isJoined(guild.uuid)}
														fallback={
															<button
																type="button"
																class="btn btn-primary btn-sm"
																onClick={() =>
																	void openPreview(guild.invite_code)
																}
															>
																查看并加入
															</button>
														}
													>
														<button
															type="button"
															class="btn btn-outline btn-sm"
															onClick={() =>
																navigate({
																	to: "/guild/$guildUUID",
																	params: { guildUUID: guild.uuid },
																})
															}
														>
															已加入
														</button>
													</Show>
												</div>
											</div>
										</div>
									</div>
								)}
							</For>
						</div>
					</Show>

					<div class="flex items-center justify-between mt-5">
						<button
							type="button"
							class="btn btn-sm"
							disabled={page() <= 1}
							onClick={() => setPage((p) => Math.max(1, p - 1))}
						>
							上一页
						</button>
						<span class="text-sm text-base-content/60">
							{page()} / {totalPages()}
						</span>
						<button
							type="button"
							class="btn btn-sm"
							disabled={page() >= totalPages()}
							onClick={() => setPage((p) => p + 1)}
						>
							下一页
						</button>
					</div>
				</Show>

				<Show when={previewOpen()}>
					<dialog
						class="modal modal-open"
						onClose={() => setPreviewOpen(false)}
					>
						<div class="modal-box">
							<h3 class="font-bold text-lg mb-4">加入服务器</h3>
							<GuildInvitePreview
								guild={previewGuild()}
								joined={previewJoined()}
								loading={previewLoading()}
								error={previewError()}
								joining={joining()}
								onConfirm={joinPreview}
								onCancel={() => setPreviewOpen(false)}
							/>
						</div>
						<form method="dialog" class="modal-backdrop">
							<button onClick={() => setPreviewOpen(false)} />
						</form>
					</dialog>
				</Show>

				<CreateGuildModal
					ref={setCreateRef}
					onClose={() => createRef()?.close()}
					onCreated={(guild) =>
						navigate({
							to: "/guild/$guildUUID",
							params: { guildUUID: guild.uuid },
						})
					}
				/>
			</div>
		</div>
	);
}

export default RouteComponent;
