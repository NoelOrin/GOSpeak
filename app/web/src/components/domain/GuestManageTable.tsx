import { For, Show } from "solid-js";
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

const GuestManageTable = (props: GuestManageTableProps) => {
	return (
		<div class="overflow-x-auto">
			<table class="table table-sm">
				<thead>
					<tr>
						<th>访客</th>
						<th>原因</th>
						<th>封禁时间</th>
						<th>到期</th>
						<Show when={props.canKick}>
							<th>操作</th>
						</Show>
					</tr>
				</thead>
				<tbody>
					<Show
						when={props.bans.length > 0}
						fallback={
							<tr>
								<td
									colspan={props.canKick ? 5 : 4}
									class="text-center text-base-content/50 py-4"
								>
									{props.loading ? "加载中…" : "暂无封禁记录"}
								</td>
							</tr>
						}
					>
						<For each={props.bans}>
							{(ban) => (
								<tr>
									<td class="font-mono text-xs">{ban.user_uuid}</td>
									<td>{ban.reason || "-"}</td>
									<td>
										{new Date(Date.parse(ban.created_at)).toLocaleString()}
									</td>
									<td>{formatBanExpiry(ban.expires_at)}</td>
									<Show when={props.canKick}>
										<td>
											<button
												type="button"
												class="btn btn-ghost btn-xs"
												disabled={props.unbanPending === ban.user_uuid}
												onClick={() => props.onUnban(ban.user_uuid)}
											>
												解封
											</button>
										</td>
									</Show>
								</tr>
							)}
						</For>
					</Show>
				</tbody>
			</table>
		</div>
	);
};

export default GuestManageTable;
