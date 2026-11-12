import { Button, toast } from "@gospeak/components";
import { useQuery } from "@tanstack/react-query";
import { Link2 } from "lucide-react";
import { useEffect, useState } from "react";
import { type DomainDetail, getMyDomainsDetailed } from "@/api/domain";
import {
	createInvite,
	type DomainInvite,
	disableInvite,
	listInvites,
} from "@/api/invite";
import { useKeepAliveActive } from "@/components/common/keepAliveOutlet";
import {
	LoadingRegion,
	SkeletonTableRows,
} from "@/components/common/Skeletons";
import DomainIcon from "@/components/domain/DomainIcon";
import InviteCodeCreateModal from "@/components/domain/InviteCodeCreateModal";
import InviteCodeTable from "@/components/domain/InviteCodeTable";
import InviteRedemptionsPanel from "@/components/domain/InviteRedemptionsPanel";
import InviteShareModal from "@/components/domain/InviteShareModal";
import { domainStore, useDomainStore } from "@/stores/domainStore";
import { domainInviteUrl } from "@/utils/domainInvite";
import { usePublicBaseUrl } from "@/utils/publicBaseUrl";
import { toastActionError } from "@/utils/toast";

export default function InviteCodeManager() {
	const isActive = useKeepAliveActive();
	const publicBase = usePublicBaseUrl();
	const myDomainUUIDs = useDomainStore((s) => s.myDomainUUIDs);
	const [activeDomain, setActiveDomain] = useState<string | undefined>();
	const [invites, setInvites] = useState<DomainInvite[]>([]);
	const [loadingInvites, setLoadingInvites] = useState(false);
	const [createOpen, setCreateOpen] = useState(false);
	const [shareInvite, setShareInvite] = useState<DomainInvite | null>(null);
	const [redemptionsInvite, setRedemptionsInvite] =
		useState<DomainInvite | null>(null);

	useEffect(() => {
		void domainStore.loadMyDomains().catch(() => {});
	}, []);

	const { data: domains, isLoading: domainsLoading } = useQuery({
		queryKey: ["my-domains-detailed-invite", myDomainUUIDs.join(",")],
		queryFn: getMyDomainsDetailed,
		enabled: isActive,
	});

	useEffect(() => {
		if (!activeDomain && domains && domains.length > 0) {
			setActiveDomain(domains[0].uuid);
		}
	}, [activeDomain, domains]);

	useEffect(() => {
		if (!activeDomain || !isActive) return;
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
	}, [activeDomain, isActive]);

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

	if (domainsLoading) {
		return <p className="text-muted-foreground text-sm">加载域列表…</p>;
	}

	if (!domains || domains.length === 0) {
		return (
			<div className="text-faint-foreground flex flex-col items-center justify-center gap-2 py-8">
				<Link2 size={24} />
				<p className="text-sm">你还没有加入任何域，暂无邀请码可管理。</p>
			</div>
		);
	}

	const current: DomainDetail | undefined = domains.find(
		(d) => d.uuid === activeDomain,
	);
	const shareUrl = shareInvite
		? domainInviteUrl(shareInvite.code, publicBase)
		: "";

	return (
		<div className="flex flex-col gap-4">
			<div className="flex flex-wrap items-center justify-between gap-3">
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
				<Button size="sm" onClick={() => setCreateOpen(true)}>
					新建邀请码
				</Button>
			</div>

			{current && (
				<div className="flex gap-4">
					<div className="min-w-0 flex-1">
						{loadingInvites ? (
							<LoadingRegion label="正在加载邀请码">
								<SkeletonTableRows rows={4} cols={4} />
							</LoadingRegion>
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
		</div>
	);
}
