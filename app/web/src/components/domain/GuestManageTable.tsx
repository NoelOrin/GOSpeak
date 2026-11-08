import type { GuestBanItem } from "@/api/guest";

interface GuestManageTableProps {
	bans: GuestBanItem[];
	loading: boolean;
	canKick: boolean;
	unbanPending: string | null;
	onUnban: (userUUID: string) => void;
}

export function formatBanExpiry(expiresAt?: string | null): string {
	if (!expiresAt) return "永久";
	const ts = Date.parse(expiresAt);
	if (Number.isNaN(ts)) return "永久";
	if (ts <= Date.now()) return "已过期";
	return new Date(ts).toLocaleString();
}

export function isBanActive(ban: GuestBanItem): boolean {
	if (!ban.expires_at) return true;
	const ts = Date.parse(ban.expires_at);
	return !Number.isNaN(ts) && ts > Date.now();
}

export default function GuestManageTable({
	bans,
	loading,
	canKick,
	unbanPending,
	onUnban,
}: GuestManageTableProps) {
	return (
		<div className="overflow-x-auto">
			<table className="w-full text-sm">
				<thead>
					<tr className="border-border border-b">
						<th className="px-3 py-2 text-left">访客</th>
						<th className="px-3 py-2 text-left">原因</th>
						<th className="px-3 py-2 text-left">封禁时间</th>
						<th className="px-3 py-2 text-left">到期</th>
						{canKick && <th className="px-3 py-2 text-left">操作</th>}
					</tr>
				</thead>
				<tbody>
					{bans.length === 0 ? (
						<tr>
							<td
								colSpan={canKick ? 5 : 4}
								className="text-faint-foreground py-4 text-center"
							>
								{loading ? "加载中…" : "暂无封禁记录"}
							</td>
						</tr>
					) : (
						bans.map((ban) => (
							<tr key={ban.user_uuid} className="border-border/60 border-b">
								<td className="px-3 py-2 font-mono text-xs">{ban.user_uuid}</td>
								<td className="px-3 py-2">{ban.reason || "-"}</td>
								<td className="px-3 py-2">
									{new Date(Date.parse(ban.created_at)).toLocaleString()}
								</td>
								<td className="px-3 py-2">{formatBanExpiry(ban.expires_at)}</td>
								{canKick && (
									<td className="px-3 py-2">
										<button
											type="button"
											className="hover:bg-muted inline-flex h-7 cursor-pointer items-center rounded-sm px-2 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50"
											disabled={unbanPending === ban.user_uuid}
											onClick={() => onUnban(ban.user_uuid)}
										>
											解封
										</button>
									</td>
								)}
							</tr>
						))
					)}
				</tbody>
			</table>
		</div>
	);
}
