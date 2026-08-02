import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import Compass from "lucide-solid/icons/compass";
import CirclePlus from "lucide-solid/icons/circle-plus";
import RefreshCw from "lucide-solid/icons/refresh-cw";
import {
	createMemo,
	createResource,
	createSignal,
	onMount,
	Show,
} from "solid-js";
import {
	type Domain,
	joinDomain,
	listPublicDomains,
	previewDomainInvite,
} from "@/api/domain";
import CreateDomainModal from "@/components/domain/CreateDomainModal";
import DomainInvitePreview from "@/components/domain/DomainInvitePreview";
import DiscoverDomainGrid from "@/components/discover/DiscoverDomainGrid";
import DiscoverJoinPanel from "@/components/discover/DiscoverJoinPanel";
import DiscoverMyDomains from "@/components/discover/DiscoverMyDomains";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import domainStore from "@/stores/domainStore";
import { extractDomainInviteCode } from "@/utils/domainInvite";

export const Route = createFileRoute("/(app)/discover/")({
	component: RouteComponent,
	staticData: {
		title: "发现域",
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
	const [previewDomain, setPreviewDomain] = createSignal<Domain | null>(null);
	const [previewCode, setPreviewCode] = createSignal("");
	const [previewError, setPreviewError] = createSignal("");
	const [joining, setJoining] = createSignal(false);

	const [createRef, setCreateRef] = createSignal<HTMLDialogElement>();

	const [publicDomains, { refetch }] = createResource(
		() => ({ keyword: keyword(), page: page(), pageSize: PAGE_SIZE }),
		({ keyword, page, pageSize }) =>
			listPublicDomains(page, pageSize, keyword || undefined),
	);

	const totalPages = createMemo(() =>
		Math.max(1, Math.ceil((publicDomains()?.total ?? 0) / PAGE_SIZE)),
	);

	const isJoined = (uuid: string) =>
		domainStore.state.myDomainUUIDs.includes(uuid);

	const previewJoined = () => {
		const domain = previewDomain();
		return !!domain && isJoined(domain.uuid);
	};

	async function openPreview(code: string) {
		setPreviewOpen(true);
		setPreviewLoading(true);
		setPreviewError("");
		setPreviewDomain(null);
		setPreviewCode(code);
		try {
			setPreviewDomain(await previewDomainInvite(code));
		} catch (e: any) {
			setPreviewError(e?.response?.data?.msg || "邀请码无效");
		} finally {
			setPreviewLoading(false);
		}
	}

	function handleInviteSubmit(e: Event) {
		e.preventDefault();
		const code = extractDomainInviteCode(inviteInput());
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
		const domain = previewDomain();
		const code = previewCode();
		if (!domain || !code) return;
		if (joining()) return;
		if (isJoined(domain.uuid)) {
			setPreviewOpen(false);
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: domain.uuid },
			});
			return;
		}
		setJoining(true);
		setPreviewError("");
		try {
			const joined = await joinDomain(code);
			domainStore.addDomain(joined);
			domainStore.setCurrentDomain(joined.uuid);
			setPreviewOpen(false);
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: joined.uuid },
			});
		} catch (e: any) {
			setPreviewError(e?.response?.data?.msg || "加入失败");
		} finally {
			setJoining(false);
		}
	}

	onMount(() => {
		domainStore.loadMyDomains();
		void (async () => {
			try {
				if (!navigator.clipboard?.readText) return;
				const text = await navigator.clipboard.readText();
				const code = extractDomainInviteCode(text);
				if (!code) return;
				setInviteInput(code);
				await openPreview(code);
			} catch {
				// 浏览器未授权读取剪贴板时静默处理，用户可手动输入。
			}
		})();
	});

	return (
		<div class="flex-1 min-w-0 h-full overflow-y-auto">
			<ManagePage class="min-h-full w-full">
				<ManageHeader
					icon={<Compass size={18} />}
					title="发现域"
					description="搜索公开语音域，或通过邀请码加入"
					actions={
						<button
							type="button"
							class="btn btn-primary btn-sm"
							onClick={() => createRef()?.showModal()}
						>
							<CirclePlus size={15} />
							新建域
						</button>
					}
				/>

				<div class="grid min-w-0 gap-5 lg:grid-cols-[minmax(340px,420px)_minmax(0,1fr)]">
					<ManageSection
						title="查找并加入"
						description="搜索公开域或粘贴邀请码"
						class="min-w-0"
					>
						<DiscoverJoinPanel
							searchInput={searchInput()}
							inviteInput={inviteInput()}
							onSearchInputChange={setSearchInput}
							onInviteInputChange={setInviteInput}
							onSearch={handleSearch}
							onInvite={handleInviteSubmit}
						/>
					</ManageSection>

					<ManageSection
						title="公开域"
						description={`${publicDomains()?.total ?? 0} 个公开域`}
						class="min-w-0"
						actions={
							<button
								type="button"
								class="btn btn-ghost btn-xs"
								disabled={publicDomains.loading}
								onClick={() => void refetch()}
							>
								{publicDomains.loading ? (
									<span class="loading loading-spinner loading-xs" />
								) : (
									<RefreshCw size={14} />
								)}
								刷新
							</button>
						}
					>
						<DiscoverDomainGrid
							domains={publicDomains()?.domains ?? []}
							loading={publicDomains.loading}
							hasError={!!publicDomains.error}
							page={page()}
							totalPages={totalPages()}
							joinedUUIDs={domainStore.state.myDomainUUIDs}
							onOpen={(domain) => void openPreview(domain.invite_code)}
							onEnter={(domain) =>
								navigate({
									to: "/domain/$domainUUID",
									params: { domainUUID: domain.uuid },
								})
							}
							onPageChange={setPage}
							onRetry={() => void refetch()}
						/>
					</ManageSection>
				</div>

				<ManageSection
					title="我的域"
					description="快捷进入已加入的语音域"
					padded={false}
				>
					<DiscoverMyDomains />
				</ManageSection>
			</ManagePage>

			<Show when={previewOpen()}>
				<dialog class="modal modal-open" onClose={() => setPreviewOpen(false)}>
					<div class="modal-box">
						<h3 class="font-bold text-lg mb-4">加入域</h3>
						<DomainInvitePreview
							domain={previewDomain()}
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

			<CreateDomainModal
				ref={setCreateRef}
				onClose={() => createRef()?.close()}
				onCreated={(domain) =>
					navigate({
						to: "/domain/$domainUUID",
						params: { domainUUID: domain.uuid },
					})
				}
			/>
		</div>
	);
}

export default RouteComponent;
