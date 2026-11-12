import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Headphones } from "lucide-react";
import { type Domain, getDomain } from "@/api/domain";
import AsyncState from "@/components/common/AsyncState";
import { SkeletonCardGrid } from "@/components/common/Skeletons";
import DomainIcon from "@/components/domain/DomainIcon";
import { domainStore, useDomainStore } from "@/stores/domainStore";

function DiscoverMyDomains() {
	const navigate = useNavigate();
	const myUUIDs = useDomainStore((s) => s.myDomainUUIDs);
	const loading = useDomainStore((s) => s.loading);

	const { data, isLoading: domainsLoading } = useQuery({
		queryKey: ["my-domains-list", myUUIDs.join(",")],
		queryFn: async (): Promise<{ domains: Domain[]; failed: number }> => {
			const results = await Promise.allSettled(
				myUUIDs.map(async (uuid) => {
					const cached = useDomainStore.getState().domainCache[uuid];
					if (cached) return cached;
					return getDomain(uuid);
				}),
			);
			const ok = results.filter(
				(r): r is PromiseFulfilledResult<Domain> => r.status === "fulfilled",
			);
			// 失败的域不能静默丢弃：否则"全部失败"时整块区域什么都不显示
			return {
				domains: ok.map((r) => r.value),
				failed: results.length - ok.length,
			};
		},
	});

	const domains = data?.domains ?? [];
	const failedCount = data?.failed ?? 0;

	const openDomain = (domain: Domain) => {
		domainStore.setCurrentDomain(domain.uuid);
		void navigate({
			to: "/domain/$domainUUID",
			params: { domainUUID: domain.uuid },
		});
	};

	return (
		<div className="p-4 md:p-5">
			<AsyncState
				compact
				loading={(loading || domainsLoading) && myUUIDs.length > 0}
				loadingText="正在加载我的域…"
				skeleton={<SkeletonCardGrid cards={2} columns={2} />}
				empty={!loading && !domainsLoading && myUUIDs.length === 0}
				emptyText={
					<span className="flex flex-col items-center gap-1 text-center">
						<span className="text-muted-foreground text-sm font-medium">
							还没有加入任何域
						</span>
						<span>
							域是独立的语音空间：在上方粘贴邀请码加入朋友的域，或点右上角「新建域」创建自己的空间。
						</span>
					</span>
				}
			>
				{failedCount > 0 && (
					<p className="text-warning mb-2 text-xs">
						{failedCount} 个域加载失败，未显示在列表中
					</p>
				)}
				{domains.length === 0 && failedCount > 0 && (
					<p className="text-faint-foreground text-sm">
						已加入的域暂时都无法加载，请稍后刷新重试
					</p>
				)}
				{domains.length > 0 && (
					<div className="grid gap-2 md:grid-cols-2">
						{domains.map((domain) => (
							<div
								key={domain.uuid}
								className="border-border/80 hover:border-primary/30 hover:bg-card flex min-w-0 items-center gap-3 rounded-lg border bg-muted/40 px-3 py-3 transition-colors"
							>
								<DomainIcon
									name={domain.name}
									iconUrl={domain.icon_url}
									className="shrink-0"
								/>
								<div className="min-w-0 flex-1">
									<div className="truncate text-sm font-medium">
										{domain.name}
									</div>
									<div className="text-faint-foreground mt-0.5 truncate text-xs">
										{domain.description || "暂无描述"}
									</div>
								</div>
								<button
									type="button"
									className="border-border hover:bg-muted inline-flex h-9 shrink-0 cursor-pointer items-center gap-1.5 rounded-sm border px-3 text-sm transition-colors"
									onClick={() => openDomain(domain)}
								>
									<Headphones size={14} />
									进入
								</button>
							</div>
						))}
					</div>
				)}
			</AsyncState>
		</div>
	);
}

export default DiscoverMyDomains;
