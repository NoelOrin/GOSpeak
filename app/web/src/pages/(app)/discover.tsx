import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Dialog, DialogContent, DialogTitle } from "@gospeak/components";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { CirclePlus, Compass, Loader2, RefreshCw } from "lucide-react";
import { useEffect, useState } from "react";
import { type Domain, listPublicDomains } from "@/api/domain";
import {
	getJoinRequest,
	joinByCode,
	joinByDomain,
	previewInvite,
} from "@/api/inviteJoin";
import { joinTarget } from "@/utils/joinTarget";
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
import { domainStore, useDomainStore } from "@/stores/domainStore";
import { extractDomainInviteCode } from "@/utils/domainInvite";
import { localizedApiMessage } from "@/utils/errorMessages";

export const Route = createFileRoute("/(app)/discover")({
	component: RouteComponent,
	staticData: {
		title: "发现域",
	},
});

const PAGE_SIZE = 12;

function RouteComponent() {
	const navigate = useNavigate();
	const _queryClient = useQueryClient();
	const [keyword, setKeyword] = useState("");
	const [searchInput, setSearchInput] = useState("");
	const [page, setPage] = useState(1);
	const [inviteInput, setInviteInput] = useState("");

	const [previewOpen, setPreviewOpen] = useState(false);
	const [previewLoading, setPreviewLoading] = useState(false);
	const [previewDomain, setPreviewDomain] = useState<Domain | null>(null);
	const [previewCode, setPreviewCode] = useState("");
	const [previewDomainUUID, setPreviewDomainUUID] = useState("");
	const [pendingDomainUUIDs, setPendingDomainUUIDs] = useState<string[]>([]);
	const [previewError, setPreviewError] = useState("");
	const [joining, setJoining] = useState(false);

	const [createOpen, setCreateOpen] = useState(false);

	const {
		data: publicDomains,
		isLoading,
		isError,
		refetch,
	} = useQuery({
		queryKey: ["public-domains", keyword, page],
		queryFn: () => listPublicDomains(page, PAGE_SIZE, keyword || undefined),
	});

	const totalPages = Math.max(
		1,
		Math.ceil((publicDomains?.total ?? 0) / PAGE_SIZE),
	);

	const isJoined = (uuid: string) =>
		useDomainStore.getState().myDomainUUIDs.includes(uuid);

	const previewJoined = () => {
		const domain = previewDomain;
		return !!domain && isJoined(domain.uuid);
	};

	async function openPreview(code: string, domainUUID = "") {
		if (previewLoading) return;
		setPreviewOpen(true);
		setPreviewLoading(true);
		setPreviewError("");
		setPreviewDomain(null);
		setPreviewCode(code);
		setPreviewDomainUUID(domainUUID);
		try {
			const data = await previewInvite(code);
			setPreviewDomain({
				id: 0,
				uuid: data.domain.uuid,
				name: data.domain.name,
				icon_url: data.domain.icon_url ?? "",
				description: data.domain.description ?? "",
				owner_uuid: "",
				is_public: true,
				require_join_approval: data.requires_approval,
				created_at: "",
			});
		} catch (e) {
			// previewInvite 已设 skipErrorToast，这里是唯一的提示出口
			setPreviewError(localizedApiMessage(e, "邀请码无效"));
		} finally {
			setPreviewLoading(false);
		}
	}

	function handleInviteSubmit(e: React.FormEvent) {
		e.preventDefault();
		const code = extractDomainInviteCode(inviteInput);
		if (!code) {
			setPreviewOpen(true);
			setPreviewError("未识别到邀请码或邀请链接");
			return;
		}
		void openPreview(code);
	}

	function handleSearch(e: React.FormEvent) {
		e.preventDefault();
		setPage(1);
		setKeyword(searchInput.trim());
	}

	async function joinPreview() {
		const domain = previewDomain;
		const code = previewCode;
		const domainUUID = previewDomainUUID;
		if (!domain) return;
		if (joining) return;
		if (isJoined(domain.uuid)) {
			setPreviewOpen(false);
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: domain.uuid },
			});
			return;
		}
		const target = joinTarget(code, domainUUID);
		if (!target) {
			setPreviewError("未识别到邀请码或邀请链接");
			return;
		}
		setJoining(true);
		setPreviewError("");
		try {
			const result =
				"domain_uuid" in target
					? await joinByDomain(target.domain_uuid)
					: await joinByCode(target.code);
			if (result.status === "pending") {
				setPreviewError("已提交加入申请，等待管理员审核");
				return;
			}
			await domainStore.loadMyDomains().catch(() => {});
			domainStore.setCurrentDomain(result.domain.uuid);
			setPreviewOpen(false);
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: result.domain.uuid },
			});
		} catch (e) {
			setPreviewError(localizedApiMessage(e, "加入失败，请稍后重试"));
		} finally {
			setJoining(false);
		}
	}

	// 对当前页的公开域批量查询自己的申请状态，用于卡片三态渲染。
	useEffect(() => {
		if (!publicDomains?.domains.length) return;
		let cancelled = false;
		void (async () => {
			const results = await Promise.all(
				(publicDomains?.domains ?? []).map(async (domain) => {
					try {
						const view = await getJoinRequest(domain.uuid);
						return view.status === "pending" ? domain.uuid : "";
					} catch {
						return "";
					}
				}),
			);
			if (cancelled) return;
			setPendingDomainUUIDs(results.filter(Boolean));
		})();
		return () => {
			cancelled = true;
		};
	}, [publicDomains]);

	useEffect(() => {
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
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	return (
		<div className="min-h-full w-full flex-1 overflow-y-auto">
			<ManagePage className="min-h-full w-full">
				<ManageHeader
					icon={<Compass size={20} />}
					title="发现域"
					description="搜索公开语音域，或通过邀请码加入"
					actions={
						<button
							type="button"
							className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-sm px-3 text-sm font-medium transition-opacity"
							onClick={() => setCreateOpen(true)}
						>
							<CirclePlus size={16} />
							新建域
						</button>
					}
				/>

				<div className="grid min-w-0 gap-5 lg:grid-cols-[minmax(340px,420px)_minmax(0,1fr)]">
					<ManageSection
						title="查找并加入"
						description="搜索公开域或粘贴邀请码"
						className="min-w-0"
					>
						<DiscoverJoinPanel
							searchInput={searchInput}
							inviteInput={inviteInput}
							onSearchInputChange={setSearchInput}
							onInviteInputChange={setInviteInput}
							onSearch={handleSearch}
							onInvite={handleInviteSubmit}
						/>
					</ManageSection>

					<ManageSection
						title="公开域"
						description={`${publicDomains?.total ?? 0} 个公开域`}
						className="min-w-0"
						actions={
							<button
								type="button"
								className="text-muted-foreground hover:bg-muted inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm px-2 text-xs transition-colors"
								disabled={isLoading}
								onClick={() => void refetch()}
							>
								{isLoading ? (
									<Loader2 className="size-3.5 animate-spin" />
								) : (
									<RefreshCw size={14} />
								)}
								刷新
							</button>
						}
					>
						<DiscoverDomainGrid
							domains={publicDomains?.domains ?? []}
							loading={isLoading}
							hasError={isError}
							page={page}
							totalPages={totalPages}
							joinedUUIDs={useDomainStore.getState().myDomainUUIDs}
							pendingUUIDs={pendingDomainUUIDs}
							onOpen={(domain) => void openPreview("", domain.uuid)}
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

			{previewOpen && (
				<DomainJoinPreviewModal
					previewDomain={previewDomain}
					previewLoading={previewLoading}
					previewError={previewError}
					previewJoined={previewJoined()}
					joining={joining}
					onConfirm={joinPreview}
					onCancel={() => setPreviewOpen(false)}
				/>
			)}

			<CreateDomainModal
				open={createOpen}
				onClose={() => setCreateOpen(false)}
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

function DomainJoinPreviewModal(props: {
	previewDomain: Domain | null;
	previewLoading: boolean;
	previewError: string;
	previewJoined: boolean;
	joining: boolean;
	onConfirm: () => void;
	onCancel: () => void;
}) {
	// 用 Radix Dialog 而不是手写遮罩：Esc 关闭、焦点陷阱、aria-labelledby 都是白拿的
	return (
		<Dialog open onOpenChange={(v) => !v && props.onCancel()}>
			<DialogContent className="max-w-md">
				<DialogTitle className="mb-1 text-lg font-bold">加入域</DialogTitle>
				<DomainInvitePreview
					domain={props.previewDomain}
					joined={props.previewJoined}
					loading={props.previewLoading}
					error={props.previewError}
					joining={props.joining}
					onConfirm={props.onConfirm}
					onCancel={props.onCancel}
				/>
			</DialogContent>
		</Dialog>
	);
}
