import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import Settings from "lucide-solid/icons/settings";
import { createEffect, createMemo, createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import ConfirmModal from "@/components/common/ConfirmModal";
import InviteShareModal from "@/components/guild/InviteShareModal";
import guildStore from "@/stores/guildStore";
import userStore from "@/stores/userStore";
import { extractGuildInviteCode, guildInviteUrl } from "@/utils/guildInvite";
import { hasPermission } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/guild/$guildUUID/")({
	component: RouteComponent,
	staticData: { title: "语音服务器", icon: "icon-channel" },
});

function apiErrorMessage(error: unknown): string {
	const response = (
		error as {
			response?: { data?: { msg?: string } };
		}
	)?.response?.data?.msg;
	if (response) return response;
	if (error instanceof Error) return error.message;
	return "请求失败";
}

function RouteComponent() {
	const params = Route.useParams();
	const navigate = useNavigate();
	const { state, setCurrentGuild, loadMembers, leaveAndClear, deleteAndClear } =
		guildStore;
	const [loading, setLoading] = createSignal(false);
	const [error, setError] = createSignal("");
	const [confirmAction, setConfirmAction] = createSignal<
		"leave" | "delete" | null
	>(null);
	const [shareRef, setShareRef] = createSignal<HTMLDialogElement>();
	let confirmDialogRef!: HTMLDialogElement;

	createEffect(() => {
		const uuid = params().guildUUID;
		setCurrentGuild(uuid);
		void loadMembers(uuid).catch(() => {});
	});

	const currentUser = () => userStore.user();
	const guild = createMemo(() => state.guildCache[params().guildUUID]);
	const isOwner = createMemo(
		() => !!currentUser() && guild()?.owner_uuid === currentUser()?.uuid,
	);
	const isJoined = createMemo(() =>
		state.myGuildUUIDs.includes(params().guildUUID),
	);
	const currentRole = createMemo(
		() =>
			state.memberCache[params().guildUUID]?.find(
				(member) => member.user_uuid === currentUser()?.uuid,
			)?.role_name,
	);
	const canManage = createMemo(
		() =>
			isOwner() || currentRole() === "admin" || hasPermission("guild:manage"),
	);
	const inviteCode = createMemo(() => {
		const code = guild()?.invite_code || "";
		return extractGuildInviteCode(code) || "";
	});
	const inviteUrl = createMemo(() => guildInviteUrl(inviteCode()));

	async function copyInviteCode() {
		const code = inviteCode();
		if (!code) return;
		try {
			await navigator.clipboard.writeText(code);
			showToast("邀请码已复制", { type: "success" });
		} catch {
			setError("复制邀请码失败，请手动复制邀请码");
			showToast("复制邀请码失败，请手动复制", { type: "error" });
		}
	}

	function requestLeave() {
		if (!isJoined() || isOwner()) return;
		setError("");
		setConfirmAction("leave");
		queueMicrotask(() => confirmDialogRef?.showModal());
	}

	function requestDelete() {
		if (!isOwner()) return;
		setError("");
		setConfirmAction("delete");
		queueMicrotask(() => confirmDialogRef?.showModal());
	}

	function closeConfirmModal() {
		if (loading()) return;
		confirmDialogRef?.close();
		setConfirmAction(null);
		setError("");
	}

	async function handleLeave() {
		if (confirmAction() !== "leave") return;
		setLoading(true);
		setError("");
		try {
			await leaveAndClear(params().guildUUID);
			setConfirmAction(null);
			navigate({ to: "/" });
		} catch (e) {
			const message = apiErrorMessage(e);
			setError(message);
			showToast(message, { type: "error" });
		} finally {
			setLoading(false);
		}
	}

	async function handleDelete() {
		if (confirmAction() !== "delete") return;
		setLoading(true);
		setError("");
		try {
			await deleteAndClear(params().guildUUID);
			setConfirmAction(null);
			navigate({ to: "/" });
		} catch (e) {
			const message = apiErrorMessage(e);
			setError(message);
			showToast(message, { type: "error" });
		} finally {
			setLoading(false);
		}
	}

	return (
		<div class="flex-1 flex flex-col p-4">
			<div class="text-2xl font-bold mb-2">{guild()?.name || "Loading..."}</div>
			<p class="text-base-content/60 mb-4">{guild()?.description || ""}</p>
			<div class="divider" />
			<div class="flex flex-wrap items-center gap-4 text-sm text-base-content/40 mb-4">
				<Show when={inviteCode()}>
					<span>
						邀请码: <code class="text-base-content/70">{inviteCode()}</code>
					</span>
					<button
						type="button"
						class="btn btn-xs btn-outline"
						onClick={copyInviteCode}
					>
						复制邀请码
					</button>
					<button
						type="button"
						class="btn btn-xs btn-primary"
						onClick={() => shareRef()?.showModal()}
					>
						分享邀请
					</button>
				</Show>
				<span>成员上限: {guild()?.max_rooms || "无限"}</span>
			</div>
			<div class="flex gap-2">
				<Show when={canManage()}>
					<button
						type="button"
						class="btn btn-sm btn-outline"
						onClick={() =>
							navigate({
								to: "/guild/$guildUUID/manage",
								params: { guildUUID: params().guildUUID },
							})
						}
					>
						<Settings size={15} />
						管理
					</button>
				</Show>
			</div>
			<Show when={guild()}>
				<Show when={error()}>
					<div class="text-error text-sm mb-2">{error()}</div>
				</Show>
				<div class="flex gap-2 mt-auto">
					<Show when={!isOwner() && isJoined()}>
						<button
							class="btn btn-error btn-sm"
							onClick={requestLeave}
							disabled={loading()}
						>
							{loading() ? "处理中..." : "离开服务器"}
						</button>
					</Show>
					<Show when={isOwner()}>
						<button
							class="btn btn-error btn-sm"
							onClick={requestDelete}
							disabled={loading()}
						>
							{loading() ? "处理中..." : "删除服务器"}
						</button>
					</Show>
				</div>
				<Show when={inviteCode()}>
					<InviteShareModal
						ref={setShareRef}
						inviteUrl={inviteUrl()}
						onClose={() => shareRef()?.close()}
					/>
				</Show>
			</Show>

			<ConfirmModal
				open={confirmAction() !== null}
				title={confirmAction() === "delete" ? "删除服务器" : "离开服务器"}
				message={
					<span>
						{confirmAction() === "delete"
							? "确定删除此服务器？此操作不可撤销。"
							: "确定离开此服务器？"}
						<Show when={error()}>
							<span class="mt-2 block text-error">{error()}</span>
						</Show>
					</span>
				}
				confirmText={confirmAction() === "delete" ? "删除" : "离开"}
				confirmClass="btn btn-error"
				loading={loading()}
				dialogRef={(el) => {
					confirmDialogRef = el;
				}}
				onClose={closeConfirmModal}
				onConfirm={() => {
					if (confirmAction() === "leave") void handleLeave();
					if (confirmAction() === "delete") void handleDelete();
				}}
			/>
		</div>
	);
}
