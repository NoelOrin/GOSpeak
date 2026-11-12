import { Button, toast } from "@gospeak/components";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Link } from "lucide-react";
import { useEffect, useState } from "react";
import { type DomainDetail, getMyDomainsDetailed } from "@/api/domain";
import {
	createInvite,
	type DomainInvite,
	disableInvite,
	listInvites,
} from "@/api/invite";
import DomainIcon from "@/components/domain/DomainIcon";
import InviteCodeCreateModal from "@/components/domain/InviteCodeCreateModal";
import InviteCodeTable from "@/components/domain/InviteCodeTable";
import InviteRedemptionsPanel from "@/components/domain/InviteRedemptionsPanel";
import InviteShareModal from "@/components/domain/InviteShareModal";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import { domainStore } from "@/stores/domainStore";
import { domainInviteUrl } from "@/utils/domainInvite";
import { usePublicBaseUrl } from "@/utils/publicBaseUrl";
import { toastActionError } from "@/utils/toast";

export const Route = createFileRoute("/(app)/link")({
	validateSearch: (search: Record<string, unknown>) => ({
		domain: typeof search.domain === "string" ? search.domain : undefined,
	}),
	component: RouteComponent,
	staticData: { title: "邀请码" },
});

function RouteComponent() {
	const search = Route.useSearch();
	const publicBase = usePublicBaseUrl();
	const [activeDomain, setActiveDomain] = useState<string | undefined>(
		search.domain,
	);
	const [invites, setInvites] = useState<DomainInvite[]>([]);
	const [loadingInvites, setLoadingInvites] = useState(false);
	const [createOpen, setCreateOpen] = useState(false);
	const [shareInvite, setShareInvite] = useState<DomainInvite | null>(null);
	const [redemptionsInvite, setRedemptionsInvite] =
		useState<DomainInvite | null>(null);

	useEffect(() => {
		void domainStore.loadMyDomains().catch(() => {});
	}, []);

	const { data: domains, isLoading } = useQuery({
		queryKey: ["my-domains-detailed-link"],
		queryFn: getMyDomainsDetailed,
	});

	useEffect(() => {
		if (!activeDomain && domains && domains.length > 0) {
			setActiveDomain(domains[0].uuid);
		}
	}, [activeDomain, domains]);

	useEffect(() => {
		if (!activeDomain) return;
		let cancelled = false;
		setLoadingInvites(true);
		void listInvites(activeDomain)
			.then((list) => {
				if (!cancelled) setInvites(list);
			})
			.catch((error) => {
				if (!cancelled) toastActionError(error, "加载邀请码失败");
			})
			.finally(() => {
				if (!cancelled) setLoadingInvites(false);
			});
		return () => {
			cancelled = true;
		};
	}, [activeDomain]);

	async function refreshInvites() {
		if (!activeDomain) return;
		try {
			setInvites(await listInvites(activeDomain));
		} catch (error) {
			toastActionError(error, "刷新邀请码失败");
		}
	}

	async function handleCreate(params: {
		label: string;
		expiresInHours: number;
		maxUses: number;
	}) {
		if (!activeDomain) return;
		try {
			await createInvite({
				domain_uuid: activeDomain,
				label: params.label,
				expires_in_hours: params.expiresInHours,
				max_uses: params.maxUses,
			});
			toast.success("邀请码已创建");
			await refreshInvites();
		} catch (error) {
			toastActionError(error, "创建邀请码失败");
		}
	}

	async function handleDisable(inviteID: number) {
		if (!activeDomain) return;
		try {
			await disableInvite(activeDomain, inviteID);
			toast.success("邀请码已禁用");
			await refreshInvites();
		} catch (error) {
			toastActionError(error, "禁用邀请码失败");
		}
	}

	if (isLoading) return <ManageLoading label="加载域列表..." />;

	const current: DomainDetail | undefined = domains?.find(
		(d) => d.uuid === activeDomain,
	);
	const shareUrl = shareInvite
		? domainInviteUrl(shareInvite.code, publicBase)
		: "";

	return (
		<ManagePage>
			<ManageHeader
				icon={<Link size={20} />}
				title="邀请码"
				description="创建、禁用邀请码并查看使用记录。"
			/>

			{domains && domains.length === 0 && (
				<ManageSection>
					<div className="text-faint-foreground flex flex-col items-center justify-center gap-2 py-10">
						<Link size={24} />
						<p className="text-sm">你还没有加入任何域，暂无邀请码可管理。</p>
					</div>
				</ManageSection>
			)}

			{domains && domains.length > 0 && (
				<ManageSection title="选择域">
					<div className="flex flex-wrap gap-2">
						{domains.map((d) => (
							<button
								key={d.uuid}
								type="button"
								onClick={() => setActiveDomain(d.uuid)}
								className={`flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm ${
									d.uuid === activeDomain
										? "border-primary bg-primary/10"
										: "border-border hover:bg-muted"
								}`}
							>
								<DomainIcon name={d.name} iconUrl={d.icon_url} />
								{d.name}
							</button>
						))}
					</div>
				</ManageSection>
			)}

			{current && (
				<ManageSection
					title="邀请码"
					actions={
						<Button size="sm" onClick={() => setCreateOpen(true)}>
							新建邀请码
						</Button>
					}
				>
					<div className="flex gap-4">
						<div className="min-w-0 flex-1">
							{loadingInvites ? (
								<p className="text-muted-foreground text-sm">加载中…</p>
							) : (
								<InviteCodeTable
									invites={invites}
									onDisable={(id) => void handleDisable(id)}
									onShowRedemptions={(invite) => setRedemptionsInvite(invite)}
									onCopy={(invite) => setShareInvite(invite)}
								/>
							)}
						</div>
						{redemptionsInvite && (
							<InviteRedemptionsPanel
								domainUUID={current.uuid}
								invite={redemptionsInvite}
								onClose={() => setRedemptionsInvite(null)}
							/>
						)}
					</div>
				</ManageSection>
			)}

			<InviteCodeCreateModal
				open={createOpen}
				onClose={() => setCreateOpen(false)}
				onCreate={handleCreate}
			/>

			{shareInvite && (
				<InviteShareModal
					open={!!shareInvite}
					inviteUrl={shareUrl}
					onClose={() => setShareInvite(null)}
				/>
			)}
		</ManagePage>
	);
}
