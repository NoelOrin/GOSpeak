import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import Settings from "lucide-solid/icons/settings";
import { createEffect, createMemo, createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import ConfirmModal from "@/components/common/ConfirmModal";
import InviteShareModal from "@/components/domain/InviteShareModal";
import domainStore from "@/stores/domainStore";
import userStore from "@/stores/userStore";
import { extractDomainInviteCode, domainInviteUrl } from "@/utils/domainInvite";
import { hasPermission } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/domain/$domainUUID/")({
	beforeLoad: ({ params }) => {
		const { domainUUID } = params;
		domainStore.activateDomain(domainUUID);
	},
	component: RouteComponent,
	staticData: { title: "语音域", icon: "icon-domain" },
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
	const { state, activateDomain, leaveAndClear, deleteAndClear } = domainStore;
	const [loading, setLoading] = createSignal(false);
	const [error, setError] = createSignal("");
	const [confirmAction, setConfirmAction] = createSignal<
		"leave" | "delete" | null
	>(null);
	const [shareRef, setShareRef] = createSignal<HTMLDialogElement>();
	let confirmDialogRef!: HTMLDialogElement;

	createEffect(() => {
		const uuid = params().domainUUID;
		activateDomain(uuid);
	});

	const currentUser = () => userStore.user();
	const domain = createMemo(() => state.domainCache[params().domainUUID]);
	const isOwner = createMemo(
		() => !!currentUser() && domain()?.owner_uuid === currentUser()?.uuid,
	);
	const isJoined = createMemo(() =>
		state.myDomainUUIDs.includes(params().domainUUID),
	);
	const currentRole = createMemo(
		() =>
			state.memberCache[params().domainUUID]?.find(
				(member) => member.user_uuid === currentUser()?.uuid,
			)?.role_name,
	);
	const canManage = createMemo(
		() =>
			isOwner() || currentRole() === "admin" || hasPermission("domain:manage"),
	);
	const inviteCode = createMemo(() => {
		const code = domain()?.invite_code || "";
		return extractDomainInviteCode(code) || "";
	});
	const inviteUrl = createMemo(() => domainInviteUrl(inviteCode()));

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
			await leaveAndClear(params().domainUUID);
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
			await deleteAndClear(params().domainUUID);
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
			<div class="text-2xl font-bold mb-2">
				{domain()?.name || "Loading..."}
			</div>
			<p class="text-base-content/60 mb-4">{domain()?.description || ""}</p>
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
			</div>
			<div class="flex gap-2">
				<Show when={canManage()}>
					<button
						type="button"
						class="btn btn-sm btn-outline"
						onClick={() =>
							navigate({
								to: "/manage/domains/$domainUUID",
								params: { domainUUID: params().domainUUID },
							})
						}
					>
						<Settings size={15} />
						管理
					</button>
				</Show>
			</div>
			<Show when={domain()}>
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
							{loading() ? "处理中..." : "离开域"}
						</button>
					</Show>
					<Show when={isOwner()}>
						<button
							class="btn btn-error btn-sm"
							onClick={requestDelete}
							disabled={loading()}
						>
							{loading() ? "处理中..." : "删除域"}
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
				title={confirmAction() === "delete" ? "删除域" : "离开域"}
				message={
					<span>
						{confirmAction() === "delete"
							? "确定删除此域？此操作不可撤销。"
							: "确定离开此域？"}
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
