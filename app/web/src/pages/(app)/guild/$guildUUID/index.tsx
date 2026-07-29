import { createFileRoute } from "@tanstack/solid-router";
import { onMount } from "solid-js";
import guildStore from "@/stores/guildStore";

export const Route = createFileRoute("/(app)/guild/$guildUUID/")({
	component: RouteComponent,
	staticData: {
		title: "语音服务器",
		icon: "icon-channel",
	},
});

function RouteComponent() {
	const { state, setCurrentGuild } = guildStore;
	const params = Route.useParams();

	onMount(() => {
		setCurrentGuild(params().guildUUID);
	});

	const guild = () => state.guildCache[params().guildUUID];

	return (
		<div class="flex-1 flex flex-col p-4">
			<div class="text-2xl font-bold mb-4">{guild()?.name || "Loading..."}</div>
			<p class="text-base-content/60 mb-4">{guild()?.description || ""}</p>
			<div class="divider" />
			<div class="text-base-content/40 mt-4">
				邀请码: {guild()?.invite_code || "-"}
			</div>
		</div>
	);
}
