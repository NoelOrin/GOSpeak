import ArrowRight from "lucide-solid/icons/arrow-right";
import Headphones from "lucide-solid/icons/headphones";
import { Show } from "solid-js";
import type { Domain } from "@/api/domain";
import DomainIcon from "@/components/domain/DomainIcon";

interface DiscoverDomainCardProps {
	domain: Domain;
	joined: boolean;
	onOpen: () => void;
	onEnter: () => void;
}

function DiscoverDomainCard(props: DiscoverDomainCardProps) {
	return (
		<div class="flex min-h-[156px] flex-col rounded-2xl border border-base-300/80 bg-base-200/30 p-4 shadow-sm transition-colors hover:border-primary/40 hover:bg-base-100">
			<div class="flex min-w-0 items-start gap-3">
				<DomainIcon
					name={props.domain.name}
					iconUrl={props.domain.icon_url}
					class="shrink-0"
				/>
				<div class="min-w-0 flex-1">
					<div class="flex items-start justify-between gap-2">
						<h4 class="min-w-0 truncate text-sm font-semibold">
							{props.domain.name}
						</h4>
						<span class="badge badge-ghost badge-sm shrink-0">公开</span>
					</div>
					<p class="mt-1 line-clamp-2 text-xs leading-5 text-base-content/60">
						{props.domain.description || "暂无描述"}
					</p>
				</div>
			</div>
			<div class="mt-auto flex items-center justify-between gap-3 border-t border-base-300/70 pt-3">
				<span class="truncate text-[11px] text-base-content/45">
					公开语音域
				</span>
				<Show
					when={props.joined}
					fallback={
						<button
							type="button"
							class="btn btn-primary btn-xs"
							onClick={props.onOpen}
						>
							<ArrowRight size={14} />
							查看并加入
						</button>
					}
				>
					<button
						type="button"
						class="btn btn-outline btn-xs"
						onClick={props.onEnter}
					>
						<Headphones size={14} />
						进入
					</button>
				</Show>
			</div>
		</div>
	);
}

export default DiscoverDomainCard;
