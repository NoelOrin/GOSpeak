import { Button } from "@gospeak/components";
import type { Domain } from "@/api/domain";
import { ManageLoading } from "@/components/manage/ManageShell";
import DiscoverDomainCard from "./DiscoverDomainCard";

interface DiscoverDomainGridProps {
	domains: Domain[];
	loading: boolean;
	hasError: boolean;
	page: number;
	totalPages: number;
	joinedUUIDs: string[];
	pendingUUIDs: string[];
	onOpen: (domain: Domain) => void;
	onEnter: (domain: Domain) => void;
	onPageChange: (page: number) => void;
	onRetry: () => void;
}

function DiscoverDomainGrid(props: DiscoverDomainGridProps) {
	const isJoined = (uuid: string) => props.joinedUUIDs.includes(uuid);

	return (
		<div className="flex min-h-64 flex-col">
			{props.loading && props.domains.length === 0 && (
				<ManageLoading label="正在加载公开域" />
			)}

			{!props.loading && props.hasError && props.domains.length === 0 && (
				<div
					role="alert"
					className="border-destructive/20 bg-destructive/10 text-destructive flex items-center justify-between gap-3 rounded-lg px-4 py-3 text-sm"
				>
					<span>公开域列表加载失败</span>
					<button
						type="button"
						className="border-border hover:bg-muted cursor-pointer rounded-sm border px-3 py-1.5 text-sm transition-colors"
						onClick={props.onRetry}
					>
						重试
					</button>
				</div>
			)}

			{props.domains.length > 0 && (
				<>
					<div className="grid gap-3 md:grid-cols-2">
						{props.domains.map((domain) => (
							<DiscoverDomainCard
								key={domain.uuid}
								domain={domain}
								joined={isJoined(domain.uuid)}
								pending={props.pendingUUIDs.includes(domain.uuid)}
								onOpen={() => props.onOpen(domain)}
								onEnter={() => props.onEnter(domain)}
							/>
						))}
					</div>

					<div className="mt-4 flex items-center justify-between gap-3">
						<Button
							type="button"
							variant="outline"
							disabled={props.page <= 1}
							onClick={() => props.onPageChange(Math.max(1, props.page - 1))}
						>
							上一页
						</Button>
						<span className="text-faint-foreground text-xs">
							{props.page} / {props.totalPages}
						</span>
						<Button
							type="button"
							variant="outline"
							disabled={props.page >= props.totalPages}
							onClick={() => props.onPageChange(props.page + 1)}
						>
							下一页
						</Button>
					</div>
				</>
			)}

			{!props.loading && !props.hasError && props.domains.length === 0 && (
				<div className="text-faint-foreground flex flex-1 items-center justify-center py-14 text-sm">
					暂无公开域
				</div>
			)}
		</div>
	);
}

export default DiscoverDomainGrid;
