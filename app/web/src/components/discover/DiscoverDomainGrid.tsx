import { For, Show } from "solid-js";
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
	onOpen: (domain: Domain) => void;
	onEnter: (domain: Domain) => void;
	onPageChange: (page: number) => void;
	onRetry: () => void;
}

function DiscoverDomainGrid(props: DiscoverDomainGridProps) {
	const isJoined = (uuid: string) => props.joinedUUIDs.includes(uuid);

	return (
		<div class="flex min-h-64 flex-col">
			<Show when={props.loading && props.domains.length === 0}>
				<ManageLoading label="正在加载公开域" />
			</Show>

			<Show
				when={!props.loading && props.hasError && props.domains.length === 0}
			>
				<div
					role="alert"
					class="flex items-center justify-between gap-3 rounded-2xl border border-error/20 bg-error/8 px-4 py-3 text-sm text-error"
				>
					<span>公开域列表加载失败</span>
					<button type="button" class="btn btn-sm" onClick={props.onRetry}>
						重试
					</button>
				</div>
			</Show>

			<Show when={props.domains.length > 0}>
				<div class="grid gap-3 md:grid-cols-2">
					<For each={props.domains}>
						{(domain) => (
							<DiscoverDomainCard
								domain={domain}
								joined={isJoined(domain.uuid)}
								onOpen={() => props.onOpen(domain)}
								onEnter={() => props.onEnter(domain)}
							/>
						)}
					</For>
				</div>

				<div class="mt-4 flex items-center justify-between gap-3">
					<button
						type="button"
						class="btn btn-sm"
						disabled={props.page <= 1}
						onClick={() => props.onPageChange(Math.max(1, props.page - 1))}
					>
						上一页
					</button>
					<span class="text-xs text-base-content/50">
						{props.page} / {props.totalPages}
					</span>
					<button
						type="button"
						class="btn btn-sm"
						disabled={props.page >= props.totalPages}
						onClick={() => props.onPageChange(props.page + 1)}
					>
						下一页
					</button>
				</div>
			</Show>

			<Show
				when={!props.loading && !props.hasError && props.domains.length === 0}
			>
				<div class="flex flex-1 items-center justify-center py-14 text-sm text-base-content/50">
					暂无公开域
				</div>
			</Show>
		</div>
	);
}

export default DiscoverDomainGrid;
