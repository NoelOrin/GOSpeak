import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Button, toast } from "@gospeak/components";
import {
	Clock,
	Gavel,
	Infinity as InfinityIcon,
	UserCheck,
	UserX,
} from "lucide-react";
import { useState } from "react";
import { cancelMute, createMute, listMutes, type MuteRecord } from "@/api/mute";
import { listUsers } from "@/api/user";
import ConfirmModal from "@/components/common/ConfirmModal";
import MuteDurationPicker from "@/components/manage/MuteDurationPicker";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
	ManageTag,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import UserSearchSelect from "@/components/manage/UserSearchSelect";
import { formatRemaining } from "@/utils/format";
import {
	invalidateQueries,
	restoreQueries,
	runOptimistic,
	snapshotQueries,
	updateQueryData,
} from "@/utils/optimistic";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";

export const Route = createFileRoute("/(app)/manage/mute/")({
	beforeLoad: () => {
		if (!hasPermission("mute:manage")) {
			denyAccess();
		}
	},
	component: MutePage,
	staticData: {
		title: "禁言",
	},
});

export default function MutePage() {
	const queryClient = useQueryClient();
	const { data: mutes = [], isLoading: mutesLoading } = useQuery({
		queryKey: ["manage-mutes-page"],
		queryFn: listMutes,
	});
	const [userSearch, setUserSearch] = useState("");
	const [userQuery, setUserQuery] = useState("");
	const { data: usersData, isLoading: usersLoading } = useQuery({
		queryKey: ["manage-mute-user-search", userQuery],
		queryFn: () => listUsers(1, 50, true, userQuery || undefined),
	});
	const [userId, setUserId] = useState<number | "">("");
	const [duration, setDuration] = useState(3600);
	const [permanent, setPermanent] = useState(false);
	const [reason, setReason] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [confirmMute, setConfirmMute] = useState(false);
	const [cancellingId, setCancellingId] = useState<number | null>(null);

	const userMap = new Map<number, string>(
		(usersData?.users ?? []).map((u) => [
			u.id,
			`${u.display_name} (${u.name})`,
		]),
	);

	// 用户页与禁言页各自维护一份禁言缓存，写操作需要同时更新、同时对账。
	const MUTE_KEYS = [["manage-mutes-page"], ["manage-mutes"]] as const;
	const snapshot = () =>
		snapshotQueries(
			queryClient,
			MUTE_KEYS.map((queryKey) => ({ queryKey })),
		);
	const updateMutes = (updater: (prev: MuteRecord[]) => MuteRecord[]) => {
		for (const queryKey of MUTE_KEYS) {
			updateQueryData<MuteRecord[]>(queryClient, { queryKey }, updater);
		}
	};
	const reconcile = () =>
		invalidateQueries(
			queryClient,
			MUTE_KEYS.map((queryKey) => ({ queryKey })),
		);

	const handleMute = async () => {
		const uid = userId;
		if (!uid) return;
		if (!permanent && duration <= 0) {
			toast.warning("请选择有效的禁言时长");
			return;
		}
		setSubmitting(true);
		const now = new Date().toISOString();
		const optimisticRecord: MuteRecord = {
			id: -uid,
			uuid: `optimistic-${uid}`,
			user_id: uid,
			muter_id: -1,
			duration,
			permanent,
			expires_at: permanent
				? null
				: new Date(Date.now() + duration * 1000).toISOString(),
			reason,
			created_at: now,
			updated_at: now,
		};
		try {
			await runOptimistic({
				snapshot,
				apply: () =>
					updateMutes((prev) => [
						...prev.filter((m) => m.user_id !== uid),
						optimisticRecord,
					]),
				restore: (snap) => restoreQueries(queryClient, snap),
				request: () =>
					createMute({ user_id: uid, duration, permanent, reason }),
				settle: reconcile,
				onError: (error) => toastActionError(error, "禁言失败"),
				onSuccess: () => {
					setUserId("");
					setDuration(3600);
					setPermanent(false);
					setReason("");
					toast.success(permanent ? "已永久禁言该用户" : "用户已被禁言");
				},
			});
		} finally {
			setSubmitting(false);
		}
	};

	const handleCancelMute = async (uid: number) => {
		setCancellingId(uid);
		try {
			await runOptimistic({
				snapshot,
				apply: () =>
					updateMutes((prev) => prev.filter((m) => m.user_id !== uid)),
				restore: (snap) => restoreQueries(queryClient, snap),
				request: () => cancelMute(uid),
				settle: reconcile,
				onError: (error) => toastActionError(error, "解除禁言失败"),
				onSuccess: () => toast.success("已解除禁言"),
			});
		} finally {
			setCancellingId(null);
		}
	};

	const iconBox =
		"border-border bg-card text-faint-foreground flex size-8 items-center justify-center rounded-lg border";

	return (
		<ManagePage>
			<ManageHeader
				icon={<Gavel size={20} />}
				title="禁言管理"
				description="查看并管理当前有效禁言"
			/>

			<ManageSection
				title="当前禁言列表"
				description={`${mutes?.length || 0} 条`}
				padded={false}
				actions={
					<span className={iconBox}>
						<UserX size={16} />
					</span>
				}
			>
				{mutesLoading ? (
					<span className="border-primary border-t-primary m-4 block size-5 animate-spin rounded-full border-2" />
				) : (mutes?.length || 0) === 0 ? (
					<div className="border-border/60 bg-muted/20 text-muted-foreground m-4 rounded-lg border border-dashed py-10 text-center text-sm">
						暂无禁言记录
					</div>
				) : (
					<div className="overflow-x-auto">
						<table className="w-full text-sm">
							<thead>
								<tr className={manageTableHeadClass}>
									<th className="px-3 py-2 text-left">用户</th>
									<th className="px-3 py-2 text-left">类型</th>
									<th className="px-3 py-2 text-left">剩余时间</th>
									<th className="px-3 py-2 text-left">原因</th>
									<th className="px-3 py-2 text-left">操作者</th>
									<th className="px-3 py-2 text-left">操作</th>
								</tr>
							</thead>
							<tbody>
								{mutes.map((mute) => (
									<tr key={mute.user_id} className={manageTableRowClass}>
										<td className="px-3 py-2 font-semibold">
											{userMap.get(mute.user_id) || `#${mute.user_id}`}
										</td>
										<td className="px-3 py-2">
											<ManageTag>
												<span className="inline-flex items-center gap-1">
													{mute.permanent ? (
														<InfinityIcon size={12} />
													) : (
														<Clock size={12} />
													)}
													{mute.permanent ? "永久" : "定时"}
												</span>
											</ManageTag>
										</td>
										<td className="text-foreground px-3 py-2 font-mono text-xs">
											{mute.permanent
												? "永久"
												: formatRemaining(mute.expires_at)}
										</td>
										<td className="text-foreground max-w-40 truncate px-3 py-2 text-xs">
											{mute.reason || "—"}
										</td>
										<td className="text-foreground px-3 py-2 text-xs">
											{userMap.get(mute.muter_id) || `#${mute.muter_id}`}
										</td>
										<td className="px-3 py-2">
											<button
												type="button"
												className="text-muted-foreground hover:bg-muted inline-flex cursor-pointer items-center gap-1 rounded-sm px-2 py-1 text-xs transition-colors disabled:pointer-events-none disabled:opacity-50"
												disabled={cancellingId === mute.user_id}
												onClick={() => void handleCancelMute(mute.user_id)}
											>
												<UserCheck size={14} />
												解除
											</button>
										</td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
				)}
			</ManageSection>

			<ManageSection
				title="添加禁言"
				description="选择用户与时长后立即生效"
				actions={
					<span className={iconBox}>
						<Gavel size={16} />
					</span>
				}
			>
				<div className="grid grid-cols-1 gap-4 xl:grid-cols-[220px_minmax(0,1fr)_240px]">
					<UserSearchSelect
						id="mute-user"
						value={userId}
						onChange={setUserId}
						users={usersData?.users || []}
						loading={usersLoading}
						disabled={submitting}
						searchValue={userSearch}
						onSearchInput={setUserSearch}
						onSearch={() => setUserQuery(userSearch.trim())}
					/>

					<div className="min-w-0">
						<div className="text-muted-foreground mb-1 py-1 text-xs font-medium">
							禁言时长
						</div>
						<MuteDurationPicker
							permanent={permanent}
							duration={duration}
							onChange={(value) => {
								setPermanent(value.permanent);
								setDuration(value.duration);
							}}
						/>
					</div>

					<div>
						<label
							className="text-muted-foreground mb-1 block py-1 text-xs font-medium"
							htmlFor="mute-reason"
						>
							原因
						</label>
						<input
							id="mute-reason"
							type="text"
							className="border-input bg-card text-foreground focus-visible:ring-ring placeholder:text-faint-foreground flex h-9 w-full rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2"
							placeholder="违规发言"
							value={reason}
							onChange={(e) => setReason(e.currentTarget.value)}
						/>
					</div>
				</div>

				<div className="mt-4 flex justify-end">
					<Button
						type="button"
						variant="destructive"
						disabled={!userId || submitting}
						onClick={() => setConfirmMute(true)}
					>
						<Gavel size={16} />
						确认禁言
					</Button>
				</div>
			</ManageSection>

			<ConfirmModal
				open={confirmMute}
				title="禁言用户"
				message={
					<span>
						确认对「{userMap.get(Number(userId)) ?? `#${userId}`}」
						{permanent
							? "永久禁言"
							: `禁言 ${Math.max(1, Math.round(duration / 60))} 分钟`}
						？该用户在禁言期间无法发言，可在上方列表提前解除。
					</span>
				}
				confirmText="禁言"
				loading={submitting}
				onClose={() => setConfirmMute(false)}
				onConfirm={() => {
					setConfirmMute(false);
					void handleMute();
				}}
			/>
		</ManagePage>
	);
}
