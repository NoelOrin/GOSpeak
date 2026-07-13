import { createFileRoute, redirect } from "@tanstack/solid-router";
import Clock from "lucide-solid/icons/clock";
import Gavel from "lucide-solid/icons/gavel";
import InfinityIcon from "lucide-solid/icons/infinity";
import UserCheck from "lucide-solid/icons/user-check";
import UserX from "lucide-solid/icons/user-x";
import { createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { cancelMute, createMute, listMutes } from "@/api/mute";
import { listUsers } from "@/api/user";
import userStore from "@/stores/userStore";
import { formatRemaining } from "@/utils/format";

export const Route = createFileRoute("/(app)/manage/mute/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
	},
	component: MutePage,
	staticData: {
		title: "禁言",
		icon: "icon-manage",
	},
});

function MutePage() {
	const [mutes, { refetch: refetchMutes }] = createResource(listMutes);
	const [users] = createResource(() => listUsers(1, 200));
	const [userId, setUserId] = createSignal<number | "">("");
	const [duration, setDuration] = createSignal(3600);
	const [permanent, setPermanent] = createSignal(false);
	const [reason, setReason] = createSignal("");
	const [submitting, setSubmitting] = createSignal(false);
	const [cancellingId, setCancellingId] = createSignal<number | null>(null);

	const userMap = () => {
		const data = users();
		if (!data) return new Map<number, string>();
		const m = new Map<number, string>();
		for (const u of data.users) {
			m.set(u.id, `${u.display_name} (${u.name})`);
		}
		return m;
	};

	const handleMute = async () => {
		const uid = userId();
		if (!uid) {
			showToast("请选择用户", { type: "warning" });
			return;
		}
		if (!permanent() && duration() <= 0) {
			showToast("请输入有效时长", { type: "warning" });
			return;
		}
		setSubmitting(true);
		try {
			await createMute({
				user_id: uid,
				duration: duration(),
				permanent: permanent(),
				reason: reason(),
			});
			showToast("禁言已创建", { type: "success" });
			setUserId("");
			setDuration(3600);
			setPermanent(false);
			setReason("");
			refetchMutes();
		} catch (error) {
			showToast(error instanceof Error ? error.message : "禁言失败", {
				type: "error",
			});
		} finally {
			setSubmitting(false);
		}
	};

	const handleCancelMute = async (uid: number) => {
		setCancellingId(uid);
		try {
			await cancelMute(uid);
			showToast("禁言已取消", { type: "success" });
			refetchMutes();
		} catch (error) {
			showToast(error instanceof Error ? error.message : "取消失败", {
				type: "error",
			});
		} finally {
			setCancellingId(null);
		}
	};

	return (
		<div class="flex h-full min-h-0 flex-col gap-4 p-4">
			<div class="flex items-center justify-between gap-3">
				<div class="flex items-center gap-2">
					<Gavel size={20} />
					<h3 class="font-bold text-lg">禁言管理</h3>
				</div>
			</div>

			<div class="min-h-0 flex-1 overflow-auto">
				<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
					<UserX size={16} />
					<span>当前禁言列表</span>
					<span class="text-base-content/50 text-xs">
						({mutes()?.length || 0} 条)
					</span>
				</div>
				<Show
					when={!mutes.loading}
					fallback={<div class="loading loading-spinner loading-sm" />}
				>
					<Show
						when={(mutes()?.length || 0) > 0}
						fallback={
							<div class="text-base-content/50 py-8 text-center text-sm">
								暂无禁言记录
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
									<For each={mutes()}>
										{(mute) => (
											<tr>
												<td>
													{userMap().get(mute.user_id) || `#${mute.user_id}`}
												</td>
												<td>
													{mute.permanent ? (
														<span class="flex items-center gap-1 text-error font-medium text-xs">
															<InfinityIcon size={13} />
															永久
														</span>
													) : (
														<span class="flex items-center gap-1 text-warning font-medium text-xs">
															<Clock size={13} />
															定时
														</span>
													)}
												</td>
												<td class="font-mono text-xs">
													{mute.permanent
														? "永久"
														: formatRemaining(mute.expires_at)}
												</td>
												<td class="max-w-40 truncate text-xs">
													{mute.reason || "—"}
												</td>
												<td class="text-xs">
													{userMap().get(mute.muter_id) || `#${mute.muter_id}`}
												</td>
												<td>
													<button
														type="button"
														class="btn btn-ghost btn-xs text-error"
														disabled={cancellingId() === mute.user_id}
														onClick={() => handleCancelMute(mute.user_id)}
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

			<div class="border-base-300 border-t" />

			<div>
				<div class="mb-3 flex items-center gap-2 font-semibold text-sm">
					<Gavel size={16} />
					<span>添加禁言</span>
				</div>
				<div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
					<div class="form-control">
						<label class="label py-1" for="mute-user">
							<span class="label-text text-xs">用户</span>
						</label>
						<select
							id="mute-user"
							class="select select-bordered select-sm"
							value={userId()}
							onChange={(e) =>
								setUserId(
									e.currentTarget.value ? Number(e.currentTarget.value) : "",
								)
							}
						>
							<option value="">选择用户</option>
							<For each={users()?.users || []}>
								{(u) => (
									<option value={u.id}>
										{u.display_name} ({u.name})
									</option>
								)}
							</For>
						</select>
					</div>

					<div class="form-control">
						<div class="label py-1">
							<span class="label-text text-xs">类型</span>
						</div>
						<div class="flex items-center gap-3 pt-1">
							<label class="flex items-center gap-1.5 text-xs">
								<input
									type="radio"
									name="mute-type"
									class="radio radio-xs"
									checked={!permanent()}
									onChange={() => setPermanent(false)}
								/>
								定时
							</label>
							<label class="flex items-center gap-1.5 text-xs">
								<input
									type="radio"
									name="mute-type"
									class="radio radio-xs"
									checked={permanent()}
									onChange={() => setPermanent(true)}
								/>
								永久
							</label>
						</div>
					</div>

					<Show when={!permanent()}>
						<div class="form-control">
							<label class="label py-1" for="mute-duration">
								<span class="label-text text-xs">时长（秒）</span>
							</label>
							<input
								id="mute-duration"
								type="number"
								class="input input-bordered input-sm"
								value={duration()}
								onInput={(e) => setDuration(Number(e.currentTarget.value) || 0)}
								min={1}
							/>
						</div>
					</Show>

					<div class="form-control">
						<label class="label py-1" for="mute-reason">
							<span class="label-text text-xs">原因</span>
						</label>
						<input
							id="mute-reason"
							type="text"
							class="input input-bordered input-sm"
							placeholder="违规发言"
							value={reason()}
							onInput={(e) => setReason(e.currentTarget.value)}
						/>
					</div>
				</div>

				<div class="mt-3 flex justify-end">
					<button
						type="button"
						class="btn btn-primary btn-sm gap-2"
						disabled={!userId() || submitting()}
						onClick={handleMute}
					>
						<Gavel size={15} />
						确认禁言
					</button>
				</div>
			</div>
		</div>
	);
}
