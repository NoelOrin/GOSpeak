import { Button, toast } from "@gospeak/components";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { ArrowRight, Check, Copy, Link, RefreshCw } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
	type DomainDetail,
	getMyDomainsDetailed,
	resetDomainInviteCode,
} from "@/api/domain";
import ConfirmModal from "@/components/common/ConfirmModal";
import DomainIcon from "@/components/domain/DomainIcon";
import InviteShareModal from "@/components/domain/InviteShareModal";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import { domainStore } from "@/stores/domainStore";
import { domainInviteUrl, extractDomainInviteCode } from "@/utils/domainInvite";
import { usePublicBaseUrl } from "@/utils/publicBaseUrl";
import { toastActionError } from "@/utils/toast";

export const Route = createFileRoute("/(app)/link")({
	validateSearch: (search: Record<string, unknown>) => ({
		domain: typeof search.domain === "string" ? search.domain : undefined,
	}),
	component: RouteComponent,
	staticData: { title: "分享链接" },
});

function RouteComponent() {
	const navigate = useNavigate();
	const _queryClient = useQueryClient();
	const search = Route.useSearch();

	useEffect(() => {
		void domainStore.loadMyDomains().catch(() => {});
	}, []);

	const {
		data: domains,
		isLoading,
		isError,
		refetch,
	} = useQuery({
		queryKey: ["my-domains-detailed-link"],
		queryFn: getMyDomainsDetailed,
	});

	const [copiedCode, setCopiedCode] = useState<string | null>(null);
	const [copiedLink, setCopiedLink] = useState<string | null>(null);
	const [shareDomain, setShareDomain] = useState<DomainDetail | null>(null);
	const [resetTarget, setResetTarget] = useState<DomainDetail | null>(null);
	const [resetting, setResetting] = useState(false);
	const copyCodeTimer = useRef<ReturnType<typeof setTimeout> | undefined>(
		undefined,
	);
	const copyLinkTimer = useRef<ReturnType<typeof setTimeout> | undefined>(
		undefined,
	);

	const inviteCode = (d: DomainDetail) =>
		extractDomainInviteCode(d.invite_code || "") || "";
	const publicBase = usePublicBaseUrl();
	const inviteUrl = (d: DomainDetail) =>
		domainInviteUrl(inviteCode(d), publicBase);

	useEffect(
		() => () => {
			if (copyCodeTimer.current !== undefined)
				clearTimeout(copyCodeTimer.current);
			if (copyLinkTimer.current !== undefined)
				clearTimeout(copyLinkTimer.current);
		},
		[],
	);

	const copy = async (text: string, kind: "code" | "link", id: string) => {
		try {
			await navigator.clipboard.writeText(text);
			toast.success(kind === "code" ? "邀请码已复制" : "邀请链接已复制");
			if (kind === "code") {
				setCopiedCode(id);
				if (copyCodeTimer.current !== undefined)
					clearTimeout(copyCodeTimer.current);
				copyCodeTimer.current = setTimeout(
					() => setCopiedCode((c) => (c === id ? null : c)),
					2000,
				);
			} else {
				setCopiedLink(id);
				if (copyLinkTimer.current !== undefined)
					clearTimeout(copyLinkTimer.current);
				copyLinkTimer.current = setTimeout(
					() => setCopiedLink((c) => (c === id ? null : c)),
					2000,
				);
			}
		} catch {
			toast.error("复制失败，请手动复制");
		}
	};

	const confirmReset = async () => {
		const d = resetTarget;
		if (!d) return;
		setResetting(true);
		try {
			const updated = await resetDomainInviteCode(d.uuid);
			domainStore.updateCachedDomain(updated);
			await refetch();
			toast.success("邀请码已重置，旧链接已失效");
			setResetTarget(null);
		} catch (e) {
			toastActionError(e, "重置邀请码失败");
		} finally {
			setResetting(false);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Link size={20} />}
				title="分享链接"
				description="管理你所在域的邀请链接，复制后分享给好友即可加入对应语音域。"
			/>

			{isLoading && <ManageLoading label="加载域列表..." />}

			{isError && (
				<div
					role="alert"
					className="border-destructive/50 bg-destructive/10 text-destructive flex items-center gap-2 rounded-sm px-3 py-2 text-sm"
				>
					加载域列表失败，请稍后重试
				</div>
			)}

			{domains &&
				(domains.length === 0 ? (
					<ManageSection>
						<div className="text-faint-foreground flex flex-col items-center justify-center gap-2 py-10">
							<Link size={24} />
							<p className="text-sm">
								你还没有加入任何域，暂无邀请链接可分享。
							</p>
						</div>
					</ManageSection>
				) : (
					domains.map((d) => {
						const code = inviteCode(d);
						const shareOpen = shareDomain?.uuid === d.uuid;
						return (
							<ManageSection
								key={d.uuid}
								className={
									search.domain === d.uuid ? "ring-primary/40 ring-2" : ""
								}
							>
								<div className="flex items-center gap-3">
									<DomainIcon name={d.name} iconUrl={d.icon_url} />
									<div className="min-w-0 flex-1">
										<div className="truncate font-semibold">{d.name}</div>
										<div className="text-faint-foreground mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
											{d.is_public && (
												<span className="border-border inline-flex items-center rounded-full border px-2 py-0.5 text-2xs">
													公开
												</span>
											)}
											<span>成员 {d.member_count}</span>
											{code && (
												<span>
													邀请码{" "}
													<code className="text-muted-foreground">{code}</code>
												</span>
											)}
										</div>
									</div>
								</div>

								<div className="mt-4 flex flex-wrap items-center gap-2">
									{code && (
										<>
											<Button
												type="button"
												variant="outline"
												className="gap-1.5"
												onClick={() => copy(code, "code", d.uuid)}
											>
												{copiedCode === d.uuid ? (
													<Check size={16} />
												) : (
													<Copy size={16} />
												)}
												{copiedCode === d.uuid ? "已复制" : "复制邀请码"}
											</Button>
											<Button
												type="button"
												variant="outline"
												className="gap-1.5"
												onClick={() => copy(inviteUrl(d), "link", d.uuid)}
											>
												{copiedLink === d.uuid ? (
													<Check size={16} />
												) : (
													<Copy size={16} />
												)}
												{copiedLink === d.uuid ? "已复制" : "复制邀请链接"}
											</Button>
											<button
												type="button"
												className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-9 cursor-pointer items-center gap-1.5 rounded-sm px-3 text-sm transition-opacity"
												onClick={() => setShareDomain(d)}
											>
												<Link size={16} />
												分享
											</button>
										</>
									)}
									<Button
										type="button"
										variant="outline"
										className="gap-1.5"
										onClick={() =>
											navigate({
												to: "/domain/$domainUUID",
												params: { domainUUID: d.uuid },
											})
										}
									>
										进入域
										<ArrowRight size={16} />
									</Button>
									<Button
										type="button"
										variant="outline"
										className="gap-1.5"
										onClick={() => setResetTarget(d)}
									>
										<RefreshCw size={16} />
										重置邀请码
									</Button>
								</div>

								<InviteShareModal
									open={shareOpen}
									inviteUrl={shareOpen ? inviteUrl(d) : ""}
									onClose={() => setShareDomain(null)}
								/>
							</ManageSection>
						);
					})
				))}

			<ConfirmModal
				open={resetTarget !== null}
				title="重置邀请码"
				message={
					<>
						确定要重置{" "}
						<span className="font-semibold">{resetTarget?.name}</span>{" "}
						的邀请码吗？旧邀请链接将立即失效，已分享的链接无法再使用。
					</>
				}
				confirmText="重置"
				confirmVariant="default"
				loading={resetting}
				onClose={() => setResetTarget(null)}
				onConfirm={confirmReset}
			/>
		</ManagePage>
	);
}
