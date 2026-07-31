import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { createSignal, onMount, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { deleteGuild, leaveGuild } from "@/api/guild";
import InviteShareModal from "@/components/guild/InviteShareModal";
import guildStore from "@/stores/guildStore";
import userStore from "@/stores/userStore";
import { guildInviteUrl } from "@/utils/guildInvite";

export const Route = createFileRoute("/(app)/guild/$guildUUID/")({
	component: RouteComponent,
	staticData: { title: "语音服务器", icon: "icon-channel" },
});

function RouteComponent() {
	const { state, setCurrentGuild, removeGuild } = guildStore;
	const params = Route.useParams();
	const navigate = useNavigate();
	const [loading, setLoading] = createSignal(false);
	const [error, setError] = createSignal("");
	const [shareRef, setShareRef] = createSignal<HTMLDialogElement>();

	onMount(() => {
		setCurrentGuild(params().guildUUID);
	});

	const guild = () => state.guildCache[params().guildUUID];
	const isOwner = () => {
		const u = userStore.user();
		return !!u && guild()?.owner_uuid === u.uuid;
	};
	const inviteUrl = () => guildInviteUrl(guild()?.invite_code || "");

	async function copyInviteCode() {
		const code = guild()?.invite_code;
		if (!code) return;
		await navigator.clipboard.writeText(code);
		showToast("邀请码已复制", { type: "success" });
	}

	async function handleLeave() {
		setLoading(true);
		setError("");
		try {
			await leaveGuild(params().guildUUID);
			removeGuild(params().guildUUID);
			navigate({ to: "/" });
		} catch (e: any) {
			setError(e?.response?.data?.msg || "离开失败");
		} finally {
			setLoading(false);
		}
	}

	async function handleDelete() {
		if (!confirm("确定删除此服务器？此操作不可撤销。")) return;
		setLoading(true);
		setError("");
		try {
			await deleteGuild(params().guildUUID);
			removeGuild(params().guildUUID);
			navigate({ to: "/" });
		} catch (e: any) {
			setError(e?.response?.data?.msg || "删除失败");
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
				<span>
					邀请码:{" "}
					<code class="text-base-content/70">
						{guild()?.invite_code || "-"}
					</code>
				</span>
				<Show when={guild()?.invite_code}>
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
			<Show when={guild()}>
				<Show when={error()}>
					<div class="text-error text-sm mb-2">{error()}</div>
				</Show>
				<div class="flex gap-2 mt-auto">
					<Show when={!isOwner()}>
						<button
							class="btn btn-error btn-sm"
							onClick={handleLeave}
							disabled={loading()}
						>
							{loading() ? "处理中..." : "离开服务器"}
						</button>
					</Show>
					<Show when={isOwner()}>
						<button
							class="btn btn-error btn-sm"
							onClick={handleDelete}
							disabled={loading()}
						>
							{loading() ? "处理中..." : "删除服务器"}
						</button>
					</Show>
				</div>
				<Show when={guild()?.invite_code}>
					<InviteShareModal
						ref={setShareRef}
						inviteUrl={inviteUrl()}
						onClose={() => shareRef()?.close()}
					/>
				</Show>
			</Show>
		</div>
	);
}
