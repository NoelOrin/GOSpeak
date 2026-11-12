import { useQuery } from "@tanstack/react-query";
import { Link, createFileRoute } from "@tanstack/react-router";
import { getDomain, myDomains } from "@/api/domain";
import AsyncState from "@/components/common/AsyncState";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";

export const Route = createFileRoute("/(app)/manage/domains/")({
	component: RouteComponent,
	staticData: { title: "域管理" },
});

function RouteComponent() {
	const {
		data: uuids,
		isLoading: uuidsLoading,
		isError: uuidsError,
		refetch: refetchUUIDs,
	} = useQuery({
		queryKey: ["manage-domains-uuids"],
		queryFn: myDomains,
	});
	const {
		data: domains,
		isLoading: domainsLoading,
		isError: domainsError,
		refetch: refetchDomains,
	} = useQuery({
		queryKey: ["manage-domains-list", uuids?.join(",")],
		queryFn: async () => {
			const ids = uuids ?? [];
			const settled = await Promise.allSettled(ids.map((id) => getDomain(id)));
			const ok = settled.filter(
				(
					r,
				): r is PromiseFulfilledResult<Awaited<ReturnType<typeof getDomain>>> =>
					r.status === "fulfilled",
			);
			// 部分失败不能静默丢弃：让调用方知道列表不完整
			return {
				domains: ok.map((r) => r.value),
				failed: settled.length - ok.length,
			};
		},
		enabled: !!uuids,
	});

	const failedCount = domains?.failed ?? 0;

	return (
		<ManagePage>
			<ManageHeader title="域管理" description="管理我加入的语音域" />
			<ManageSection title="我的域" padded={false}>
				<AsyncState
					loading={uuidsLoading || domainsLoading}
					loadingText="正在加载你的域…"
					error={
						uuidsError || domainsError
							? "域列表加载失败，请检查网络后重试"
							: null
					}
					onRetry={() => {
						void refetchUUIDs();
						void refetchDomains();
					}}
					empty={!!domains && domains.domains.length === 0}
					emptyText="暂无域，去「发现域」创建或加入一个"
					compact
				>
					{failedCount > 0 && (
						<div className="border-warning/30 bg-warning/10 text-warning mx-4 mt-3 rounded-sm border px-3 py-2 text-xs">
							{failedCount} 个域的信息加载失败，列表中未显示
						</div>
					)}
					{domains && domains.domains.length > 0 && (
						<div className="divide-border/60 divide-y">
							{domains.domains.map((domain) => (
								<div
									key={domain.uuid}
									className="flex items-center justify-between gap-3 px-4 py-3"
								>
									<div className="min-w-0">
										<div className="truncate font-medium">{domain.name}</div>
										<div className="text-faint-foreground truncate text-xs">
											{domain.description || "暂无描述"}
										</div>
									</div>
									<Link
										to="/manage/domains/$domainUUID"
										params={{ domainUUID: domain.uuid }}
										className="border-border hover:bg-muted inline-flex h-9 shrink-0 items-center rounded-sm border px-3 text-sm transition-colors"
									>
										管理
									</Link>
								</div>
							))}
						</div>
					)}
				</AsyncState>
			</ManageSection>
		</ManagePage>
	);
}
