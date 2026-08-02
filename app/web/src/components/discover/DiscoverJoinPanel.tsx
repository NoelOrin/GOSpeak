import Link2 from "lucide-solid/icons/link-2";
import Search from "lucide-solid/icons/search";

interface DiscoverJoinPanelProps {
	searchInput: string;
	inviteInput: string;
	onSearchInputChange: (value: string) => void;
	onInviteInputChange: (value: string) => void;
	onSearch: (event: Event) => void;
	onInvite: (event: Event) => void;
}

function DiscoverJoinPanel(props: DiscoverJoinPanelProps) {
	return (
		<div class="flex flex-col gap-4">
			<form onSubmit={props.onSearch} class="flex flex-col gap-2">
				<label class="form-control">
					<span class="label px-0 pb-1.5">
						<span class="label-text text-xs font-medium text-base-content/70">
							搜索公开域
						</span>
					</span>
					<span class="relative block">
						<Search class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-base-content/45" />
						<input
							type="search"
							class="input input-bordered w-full pl-9"
							value={props.searchInput}
							onInput={(e) => props.onSearchInputChange(e.currentTarget.value)}
							placeholder="搜索域名称或描述"
						/>
					</span>
				</label>
				<button type="submit" class="btn btn-primary btn-sm justify-start">
					<Search size={15} />
					搜索
				</button>
			</form>

			<div class="flex items-center gap-3 text-[11px] font-medium uppercase tracking-[0.16em] text-base-content/40">
				<span class="h-px flex-1 bg-base-300/80" />
				或
				<span class="h-px flex-1 bg-base-300/80" />
			</div>

			<form onSubmit={props.onInvite} class="flex flex-col gap-2">
				<label class="form-control">
					<span class="label px-0 pb-1.5">
						<span class="label-text text-xs font-medium text-base-content/70">
							邀请码加入
						</span>
					</span>
					<span class="relative block">
						<Link2 class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-base-content/45" />
						<input
							type="text"
							class="input input-bordered w-full pl-9"
							value={props.inviteInput}
							onInput={(e) => props.onInviteInputChange(e.currentTarget.value)}
							placeholder="粘贴邀请码或邀请链接"
						/>
					</span>
				</label>
				<button type="submit" class="btn btn-outline btn-sm justify-start">
					<Link2 size={15} />
					查看邀请
				</button>
			</form>
		</div>
	);
}

export default DiscoverJoinPanel;
