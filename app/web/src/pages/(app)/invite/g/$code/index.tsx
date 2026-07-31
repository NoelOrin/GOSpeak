import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { createMemo, createResource, createSignal, Show } from "solid-js";
import { joinGuild, previewGuildInvite } from "@/api/guild";
import GuildIcon from "@/components/guild/GuildIcon";
import guildStore from "@/stores/guildStore";

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
	const [guild] = createResource(() => params().code, previewGuildInvite);
	const [joining, setJoining] = createSignal(false);
	const [error, setError] = createSignal("");

	const joined = createMemo(() => {
		const current = guild();
		return !!current && guildStore.state.myGuildUUIDs.includes(current.uuid);
	});

	async function handleJoin() {
		const current = guild();
		if (!current) return;
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
						when={!guild.loading}
						fallback={
							<div class="flex justify-center py-8">
								<span class="loading loading-spinner loading-md" />
							</div>
						}
					>
						<Show
							when={!guild.error && guild()}
							fallback={
								<div class="alert alert-error">
									<span>邀请链接无效或服务器不存在</span>
								</div>
							}
						>
							{(guild) => (
								<>
									<div class="flex items-start gap-3">
										<GuildIcon
											name={guild().name}
											iconUrl={guild().icon_url}
											class="shrink-0"
										/>
										<div class="min-w-0">
											<h1 class="text-xl font-bold truncate">{guild().name}</h1>
											<p class="mt-1 text-sm text-base-content/60">
												{guild().description || "暂无描述"}
											</p>
										</div>
									</div>
									<Show when={error()}>
										<div class="alert alert-error mt-4">
											<span>{error()}</span>
										</div>
									</Show>
									<div class="modal-action">
										<Show
											when={joined()}
											fallback={
												<button
													type="button"
													class="btn btn-primary"
													disabled={joining()}
													onClick={handleJoin}
												>
													{joining() ? "加入中..." : "确认加入"}
												</button>
											}
										>
											<button
												type="button"
												class="btn btn-primary"
												onClick={handleJoin}
											>
												进入服务器
											</button>
										</Show>
									</div>
								</>
							)}
						</Show>
					</Show>
				</div>
			</div>
		</div>
	);
}

export default RouteComponent;
