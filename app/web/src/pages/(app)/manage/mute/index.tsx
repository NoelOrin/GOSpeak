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
import {
	ManageHeader,
	ManagePage,
	ManageSection,
	ManageTag,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import UserSearchSelect from "@/components/manage/UserSearchSelect";
import MuteDurationPicker from "@/components/manage/MuteDurationPicker";
import { formatRemaining } from "@/utils/format";
import { hasPermission } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/manage/mute/")({
	beforeLoad: () => {
		if (!hasPermission("mute:manage")) {
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
	const [userSearch, setUserSearch] = createSignal("");
	const [userQuery, setUserQuery] = createSignal("");
	const [users] = createResource(
		() => userQuery().trim(),
		(keyword) => listUsers(1, 50, true, keyword || undefined),
	);
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
		} catch {
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
		} catch {
		} finally {
			setCancellingId(null);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Gavel size={18} />}
				title="禁言管理"
				description="查看并管理当前有效禁言"
			/>

			<ManageSection
				title="当前禁言列表"
				description={`${mutes()?.length || 0} 条`}
				padded={false}
				actions={
					<span class="flex size-8 items-center justify-center rounded-lg border border-base-300 bg-base-100 text-base-content/50">
						<UserX size={16} />
					</span>
				}
			>
				<Show
					when={!mutes.loading}
					fallback={<div class="loading loading-spinner loading-sm m-4" />}
				>
					<Show
						when={(mutes()?.length || 0) > 0}
						fallback={
							<div class="m-4 rounded-xl border border-dashed border-base-300 bg-base-200/20 py-10 text-center text-sm text-base-content/55">
								暂无禁言记录
							</div>
						}
					>
						<div class="overflow-x-auto">
							<table class="table table-sm">
								<thead>
									<tr class={manageTableHeadClass}>
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
											<tr class={manageTableRowClass}>
												<td class="font-semibold">
													{userMap().get(mute.user_id) || `#${mute.user_id}`}
												</td>
												<td>
													<ManageTag>
														<span class="inline-flex items-center gap-1">
															{mute.permanent ? (
																<InfinityIcon size={12} />
															) : (
																<Clock size={12} />
															)}
															{mute.permanent ? "永久" : "定时"}
														</span>
													</ManageTag>
												</td>
												<td class="font-mono text-xs text-base-content/75">
													{mute.permanent
														? "永久"
														: formatRemaining(mute.expires_at)}
												</td>
												<td class="max-w-40 truncate text-xs text-base-content/75">
													{mute.reason || "—"}
												</td>
												<td class="text-xs text-base-content/75">
													{userMap().get(mute.muter_id) || `#${mute.muter_id}`}
												</td>
												<td>
													<button
														type="button"
														class="btn btn-ghost btn-xs text-base-content/60"
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
			</ManageSection>

			<ManageSection
				title="添加禁言"
				description="选择用户与时长后立即生效"
				actions={
					<span class="flex size-8 items-center justify-center rounded-lg border border-base-300 bg-base-100 text-base-content/50">
						<Gavel size={16} />
					</span>
				}
			>
				<div class="grid grid-cols-1 gap-4 xl:grid-cols-[220px_minmax(0,1fr)_240px]">
					<UserSearchSelect
						id="mute-user"
						value={userId()}
						onChange={setUserId}
						users={users()?.users || []}
						loading={users.loading}
						disabled={submitting()}
						searchValue={userSearch()}
						onSearchInput={setUserSearch}
						onSearch={() => setUserQuery(userSearch().trim())}
					/>

					<div class="form-control min-w-0">
						<div class="label py-1">
							<span class="label-text text-xs font-medium text-base-content/70">
								禁言时长
							</span>
						</div>
						<MuteDurationPicker
							permanent={permanent()}
							duration={duration()}
							onChange={(value) => {
								setPermanent(value.permanent);
								setDuration(value.duration);
							}}
						/>
					</div>

					<div class="form-control">
						<label class="label py-1" for="mute-reason">
							<span class="label-text text-xs font-medium text-base-content/70">
								原因
							</span>
						</label>
						<input
							id="mute-reason"
							type="text"
							class="input input-bordered input-sm bg-base-100 placeholder:text-base-content/40"
							placeholder="违规发言"
							value={reason()}
							onInput={(e) => setReason(e.currentTarget.value)}
						/>
					</div>
				</div>

				<div class="mt-4 flex justify-end">
					<button
						type="button"
						class="btn btn-sm gap-2 border border-base-300 bg-base-100 text-base-content shadow-none hover:bg-base-200"
						disabled={!userId() || submitting()}
						onClick={handleMute}
					>
						<Gavel size={15} />
						确认禁言
					</button>
				</div>
			</ManageSection>
		</ManagePage>
	);
}
