import { ArrowRight, Clock, Headphones } from "lucide-react";
import type { Domain } from "@/api/domain";
import DomainIcon from "@/components/domain/DomainIcon";

interface DiscoverDomainCardProps {
	domain: Domain;
	joined: boolean;
	pending: boolean;
	onOpen: () => void;
	onEnter: () => void;
}

function DiscoverDomainCard(props: DiscoverDomainCardProps) {
	return (
		<div className="border-border/80 hover:border-primary/40 hover:bg-card flex min-h-[156px] flex-col rounded-lg border bg-muted/30 p-4 shadow-sm transition-colors">
			<div className="flex min-w-0 items-start gap-3">
				<DomainIcon
					name={props.domain.name}
					iconUrl={props.domain.icon_url}
					className="shrink-0"
				/>
				<div className="min-w-0 flex-1">
					<div className="flex items-start justify-between gap-2">
						<h4 className="min-w-0 truncate text-sm font-semibold">
							{props.domain.name}
						</h4>
						<span className="bg-muted text-foreground shrink-0 rounded-full px-2 py-0.5 text-2xs">
							公开
						</span>
					</div>
					<p className="text-muted-foreground mt-1 line-clamp-2 text-xs leading-5">
						{props.domain.description || "暂无描述"}
					</p>
				</div>
			</div>
			<div className="border-border/70 mt-auto flex items-center justify-between gap-3 border-t pt-3">
				<span className="text-faint-foreground truncate text-2xs">
					公开语音域
				</span>
				{props.joined ? (
					<button
						type="button"
						className="border-border hover:bg-muted inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm border px-2 text-xs transition-colors"
						onClick={props.onEnter}
					>
						<Headphones size={14} />
						进入
					</button>
				) : props.pending ? (
					<button
						type="button"
						disabled
						className="border-border text-muted-foreground inline-flex h-7 items-center gap-1 rounded-sm border px-2 text-xs"
					>
						<Clock size={14} />
						申请审核中
					</button>
				) : (
					<button
						type="button"
						className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-7 cursor-pointer items-center gap-1 rounded-sm px-2 text-xs transition-opacity"
						onClick={props.onOpen}
					>
						<ArrowRight size={14} />
						查看并加入
					</button>
				)}
			</div>
		</div>
	);
}

export default DiscoverDomainCard;
