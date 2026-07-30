import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { createSignal, onMount, Show } from "solid-js";
import { deleteGuild, leaveGuild } from "@/api/guild";
import guildStore from "@/stores/guildStore";
import userStore from "@/stores/userStore";

export const Route = createFileRoute("/(app)/guild/$guildUUID/")({
	component: RouteComponent,
	staticData: { title: "语音服务器", icon: "icon-channel" },
});

function RouteComponent() {
	const { state, setCurrentGuild, removeGuild } = guildStore;
	const params = Route.useParams();
	const navigate = useNavigate();
	const [loading, setLoading] = createSignal(false);

	onMount(() => {
		setCurrentGuild(params().guildUUID);
	});

	const guild = () => state.guildCache[params().guildUUID];
	const isOwner = () => {
		const u = userStore.user();
		return !!u && guild()?.owner_uuid === u.uuid;
	};

	async function handleLeave() {
		setLoading(true);
		try {
			await leaveGuild(params().guildUUID);
			removeGuild(params().guildUUID);
			navigate({ to: "/" });
		} finally {
			setLoading(false);
		}
	}

	async function handleDelete() {
		if (!confirm("确定删除此服务器？此操作不可撤销。")) return;
		setLoading(true);
		try {
			await deleteGuild(params().guildUUID);
			removeGuild(params().guildUUID);
			navigate({ to: "/" });
		} finally {
			setLoading(false);
		}
	}

	return (
		<div class="flex-1 flex flex-col p-4">
			<div class="text-2xl font-bold mb-2">{guild()?.name || "Loading..."}</div>
			<p class="text-base-content/60 mb-4">{guild()?.description || ""}</p>
			<div class="divider" />
			<div class="flex items-center gap-4 text-sm text-base-content/40 mb-4">
				<span>
					邀请码:{" "}
					<code class="text-base-content/70">
						{guild()?.invite_code || "-"}
					</code>
				</span>
				<span>成员上限: {guild()?.max_rooms || "无限"}</span>
			</div>
			<div class="flex gap-2 mt-auto">
				<Show when={!isOwner()}>
					<button
						class="btn btn-error btn-sm"
						onClick={handleLeave}
						disabled={loading()}
					>
						离开服务器
					</button>
				</Show>
				<Show when={isOwner()}>
					<button
						class="btn btn-error btn-sm"
						onClick={handleDelete}
						disabled={loading()}
					>
						删除服务器
					</button>
				</Show>
			</div>
		</div>
	);
}
