import UserCheck from "lucide-solid/icons/user-check";
import UserX from "lucide-solid/icons/user-x";
import { For, Show } from "solid-js";
import type { MuteRecord } from "@/api/mute";

export interface ActiveMuteListProps {
	loading: boolean;
	mutes: MuteRecord[];
	userMap: Map<number, string>;
	cancellingId: number | null;
	formatRemaining: (expiresAt: string | null) => string;
	onCancel: (userId: number) => void;
}

export default function ActiveMuteList(props: ActiveMuteListProps) {
	return (
		<div>
			<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
				<UserX size={16} />
				<span>活跃禁言</span>
				<span class="text-base-content/50 text-xs">
					({props.mutes.length || 0} 条)
				</span>
			</div>
			<Show
				when={!props.loading}
				fallback={<div class="loading loading-spinner loading-sm" />}
			>
				<Show
					when={(props.mutes.length || 0) > 0}
					fallback={
						<div class="text-base-content/50 py-4 text-center text-sm">
							暂无活跃禁言
						</div>
					}
				>
					<div class="overflow-x-auto">
						<table class="table table-zebra table-xs">
							<thead>
								<tr>
									<th>用户</th>
									<th>类型</th>
									<th>剩余时间</th>
									<th>原因</th>
									<th>操作者</th>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								<For each={props.mutes}>
									{(mute) => (
										<tr>
											<td class="font-medium">
												{props.userMap.get(mute.user_id) || `#${mute.user_id}`}
											</td>
											<td>
												{mute.permanent ? (
													<span class="badge badge-error badge-xs">永久</span>
												) : (
													<span class="badge badge-warning badge-xs">定时</span>
												)}
											</td>
											<td class="text-xs">
												{mute.permanent
													? "—"
													: props.formatRemaining(mute.expires_at ?? null)}
											</td>
											<td class="max-w-40 truncate text-xs">
												{mute.reason || "—"}
											</td>
											<td class="text-xs">
												{props.userMap.get(mute.muter_id) ||
													`#${mute.muter_id}`}
											</td>
											<td>
												<button
													type="button"
													class="btn btn-ghost btn-xs text-error"
													disabled={props.cancellingId === mute.user_id}
													onClick={() => props.onCancel(mute.user_id)}
												>
													<UserCheck size={14} />
													解除
												</button>
											</td>
										</tr>
									)}
								</For>
							</tbody>
						</table>
					</div>
				</Show>
			</Show>
		</div>
	);
}
