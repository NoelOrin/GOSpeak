import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { createMemo, createResource, createSignal, Show } from "solid-js";
import { joinGuild, previewGuildInvite } from "@/api/guild";
import GuildInvitePreview from "@/components/guild/GuildInvitePreview";
import guildStore from "@/stores/guildStore";
import { extractGuildInviteCode } from "@/utils/guildInvite";

export const Route = createFileRoute("/(app)/invite/g/$code/")({
	component: RouteComponent,
	staticData: {
		title: "邀请加入",
		icon: "link",
	},
});

function RouteComponent() {
	const params = Route.useParams();
	const navigate = useNavigate();
	const inviteCode = createMemo(() => extractGuildInviteCode(params().code));
	const [guild] = createResource(inviteCode, (code) => {
		if (!code) throw new Error("invalid invite code");
		return previewGuildInvite(code);
	});
	const [joining, setJoining] = createSignal(false);
	const [error, setError] = createSignal("");

	const joined = createMemo(() => {
		const current = guild();
		return !!current && guildStore.state.myGuildUUIDs.includes(current.uuid);
	});

	async function handleJoin() {
		const current = guild();
		if (!current) return;
		if (joining()) return;
		if (joined()) {
			navigate({
				to: "/guild/$guildUUID",
				params: { guildUUID: current.uuid },
			});
			return;
		}
		setJoining(true);
		setError("");
		try {
			const joinedGuild = await joinGuild(current.invite_code);
			guildStore.addGuild(joinedGuild);
			guildStore.setCurrentGuild(joinedGuild.uuid);
			navigate({
				to: "/guild/$guildUUID",
				params: { guildUUID: joinedGuild.uuid },
			});
		} catch (e: any) {
			setError(e?.response?.data?.msg || "加入失败");
		} finally {
			setJoining(false);
		}
	}

	return (
		<div class="flex-1 flex items-center justify-center p-4 overflow-y-auto">
			<div class="card w-full max-w-md bg-base-100 border border-base-300">
				<div class="card-body">
					<Show
						when={inviteCode()}
						fallback={
							<div class="alert alert-error">
								<span>邀请链接无效或服务器不存在</span>
							</div>
						}
					>
						<GuildInvitePreview
							guild={guild()}
							joined={joined()}
							loading={guild.loading}
							error={guild.error ? "邀请链接无效或服务器不存在" : error()}
							joining={joining()}
							onConfirm={handleJoin}
						/>
					</Show>
				</div>
			</div>
		</div>
	);
}

export default RouteComponent;
