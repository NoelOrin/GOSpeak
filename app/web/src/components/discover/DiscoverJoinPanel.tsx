import { Button, Input } from "@gospeak/components";
import { Link2, Search } from "lucide-react";

interface DiscoverJoinPanelProps {
	searchInput: string;
	inviteInput: string;
	onSearchInputChange: (value: string) => void;
	onInviteInputChange: (value: string) => void;
	onSearch: (event: React.FormEvent) => void;
	onInvite: (event: React.FormEvent) => void;
}

function DiscoverJoinPanel(props: DiscoverJoinPanelProps) {
	return (
		<div className="flex flex-col gap-4">
			<form onSubmit={props.onSearch} className="flex flex-col gap-2">
				<label className="block">
					<span className="text-muted-foreground mb-1.5 block text-xs font-medium">
						搜索公开域
					</span>
					<span className="relative block">
						<Search className="text-faint-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
						<Input
							type="search"
							className="pl-9"
							value={props.searchInput}
							onChange={(e) => props.onSearchInputChange(e.currentTarget.value)}
							placeholder="搜索域名称或描述"
						/>
					</span>
				</label>
				<button
					type="submit"
					className="bg-primary text-primary-foreground hover:opacity-90 inline-flex h-9 cursor-pointer items-center justify-start gap-2 rounded-sm px-3 text-sm font-medium transition-opacity"
				>
					<Search size={16} />
					搜索
				</button>
			</form>

			<div className="text-faint-foreground flex items-center gap-3 text-2xs font-medium tracking-[0.16em] uppercase">
				<span className="bg-border/80 h-px flex-1" />
				或
				<span className="bg-border/80 h-px flex-1" />
			</div>

			<form onSubmit={props.onInvite} className="flex flex-col gap-2">
				<label className="block">
					<span className="text-muted-foreground mb-1.5 block text-xs font-medium">
						邀请码加入
					</span>
					<span className="relative block">
						<Link2 className="text-faint-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
						<Input
							type="text"
							className="pl-9"
							value={props.inviteInput}
							onChange={(e) => props.onInviteInputChange(e.currentTarget.value)}
							placeholder="粘贴邀请码或邀请链接"
						/>
					</span>
				</label>
				<Button type="submit" variant="outline" className="justify-start">
					<Link2 size={16} />
					查看邀请
				</Button>
			</form>
		</div>
	);
}

export default DiscoverJoinPanel;
