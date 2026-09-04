import UserCheck from "lucide-solid/icons/user-check";
import { For, Show } from "solid-js";
import type { MuteRecord } from "@/api/mute";
import {
	ManageSection,
	ManageTag,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";

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
		<ManageSection
			title="活跃禁言"
			description={`${props.mutes.length || 0} 条`}
			padded={false}
		>
			<Show
				when={!props.loading}
				fallback={<div class="loading loading-spinner loading-sm m-4" />}
			>
				<Show
					when={(props.mutes.length || 0) > 0}
					fallback={
						<div class="m-4 rounded-xl border border-dashed border-base-300 bg-base-200/20 py-10 text-center text-sm text-base-content/55">
							暂无活跃禁言
						</div>
					}
				>
					<div class="overflow-x-auto">
						<table class="table table-sm">
							<thead>
								<tr class={manageTableHeadClass}>
									<th>用户</th>
									<th class="w-20">类型</th>
									<th class="w-32">剩余时间</th>
									<th>原因</th>
									<th class="w-32">操作者</th>
									<th class="w-24">操作</th>
								</tr>
							</thead>
							<tbody>
								<For each={props.mutes}>
									{(mute) => (
										<tr class={manageTableRowClass}>
											<td class="font-medium">
												{props.userMap.get(mute.user_id) || `#${mute.user_id}`}
											</td>
											<td>
												<ManageTag>
													{mute.permanent ? "永久" : "定时"}
												</ManageTag>
											</td>
											<td class="text-sm text-base-content/70">
												{mute.permanent
													? "—"
													: props.formatRemaining(mute.expires_at ?? null)}
											</td>
											<td class="max-w-48 truncate text-sm text-base-content/70">
												{mute.reason || "—"}
											</td>
											<td class="text-sm text-base-content/70">
												{props.userMap.get(mute.muter_id) ||
													`#${mute.muter_id}`}
											</td>
											<td>
												<button
													type="button"
													class="btn btn-ghost btn-xs gap-1 text-base-content/60"
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
		</ManageSection>
	);
}
