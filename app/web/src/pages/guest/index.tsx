import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { guestJoin, type GuestDomain } from "@/api/guest";
import { listPublicDomains } from "@/api/domain";
import userStore from "@/stores/userStore";
import { setGuestCaps } from "@/stores/guestStore";

export const Route = createFileRoute("/guest/")({
	validateSearch: (search: Record<string, unknown>) => ({
		code:
			typeof search.code === "string" && search.code ? search.code : undefined,
		domain:
			typeof search.domain === "string" && search.domain
				? search.domain
				: undefined,
	}),
	component: GuestPage,
});

function applyGuestDomainCaps(domain?: GuestDomain | null) {
	if (!domain) return;
	setGuestCaps({
		listen: domain.guest_can_listen ?? true,
		speak: domain.guest_can_speak ?? true,
		message: domain.guest_can_message ?? false,
	});
}

function GuestPage() {
	const navigate = useNavigate();
	const search = Route.useSearch();
	const inviteCode = () => search().code ?? "";
	const domainUUID = () => search().domain ?? "";

	const [publicDomains] = createResource(async () => {
		if (inviteCode() || domainUUID()) return [];
		try {
			const page = await listPublicDomains(1, 50);
			return (page.domains ?? []).filter((d) => d.allow_guest);
		} catch {
			return [];
		}
	});

	const [nickname, setNickname] = createSignal("");
	const [selected, setSelected] = createSignal<GuestDomain | null>(null);
	const [submitting, setSubmitting] = createSignal(false);

	async function enter() {
		const name = nickname().trim();
		if (!name) {
			showToast("请输入昵称", { type: "error" });
			return;
		}
		setSubmitting(true);
		try {
			const data = await guestJoin({
				nickname: name,
				invite_code: inviteCode() || undefined,
				domain_uuid: domainUUID() || (selected()?.uuid as string | undefined),
			});
			await userStore.login({
				id: data.user.id,
				uuid: data.user.uuid,
				name: data.user.name,
				display_name: data.user.display_name,
				avatar: data.user.avatar,
				role: data.user.role,
				is_guest: true,
			});
			applyGuestDomainCaps(data.domain);
			showToast(`欢迎，${data.user.display_name}`, { type: "success" });
			navigate({ to: "/" });
		} catch (e) {
			const code = (
				e as { response?: { data?: { code?: number; msg?: string } } }
			)?.response?.data;
			if (code?.code === 1013) {
				showToast("该域未开放访客进入", { type: "error" });
			} else if (code?.code === 1017) {
				showToast(code.msg || "访客人数已达上限或操作过于频繁", {
					type: "error",
				});
			}
			// 其他错误由拦截器统一 toast
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<div class="flex justify-center items-center w-screen h-screen bg-base-200">
			<div class="card bg-base-100 shadow-xl w-full max-w-md mx-4">
				<div class="card-body gap-4">
					<h1 class="card-title text-xl">以访客身份进入</h1>

					<Show when={userStore.isLoggedIn() && userStore.user()?.is_guest}>
						<button
							type="button"
							class="btn btn-primary"
							onClick={() => navigate({ to: "/" })}
						>
							以 {userStore.user()?.display_name} 继续
						</button>
						<div class="divider text-xs text-base-content/50">或换一个身份</div>
					</Show>

					<label class="form-control">
						<span class="label-text mb-1">昵称（最多 24 字符）</span>
						<input
							class="input input-bordered w-full"
							maxLength={24}
							value={nickname()}
							placeholder="输入昵称"
							onInput={(e) => setNickname(e.currentTarget.value)}
							onKeyDown={(e) => {
								if (e.key === "Enter") void enter();
							}}
						/>
					</label>

					<Show when={!inviteCode()}>
						<Show
							when={domainUUID()}
							fallback={
								<Show when={(publicDomains()?.length ?? 0) > 0}>
									<div class="flex flex-wrap gap-2">
										<For each={publicDomains()}>
											{(d) => (
												<button
													type="button"
													class={`btn btn-sm ${
														selected()?.uuid === d.uuid
															? "btn-primary"
															: "btn-outline"
													}`}
													onClick={() => setSelected(d)}
												>
													{d.name}
												</button>
											)}
										</For>
									</div>
								</Show>
							}
						>
							<div class="text-sm text-base-content/60">
								将通过公开域加入：{domainUUID()}
							</div>
						</Show>
					</Show>
					<Show when={inviteCode()}>
						<div class="text-sm text-base-content/60">使用邀请链接加入</div>
					</Show>

					<button
						type="button"
						class="btn btn-primary mt-2"
						disabled={submitting()}
						onClick={() => void enter()}
					>
						<Show when={submitting()} fallback={<span>进入</span>}>
							<span class="loading loading-spinner loading-xs" />
						</Show>
					</button>
				</div>
			</div>
		</div>
	);
}
