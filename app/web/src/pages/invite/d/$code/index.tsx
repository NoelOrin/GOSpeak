import { createFileRoute, redirect, useNavigate } from "@tanstack/solid-router";
import Link2Off from "lucide-solid/icons/link-2-off";
import {
	createMemo,
	createResource,
	createSignal,
	Match,
	onMount,
	Show,
	Switch,
} from "solid-js";
import { joinDomain, previewDomainInvite } from "@/api/domain";
import {
	getDomainInviteAction,
	getDomainInvitePreviewStatus,
} from "@/components/domain/DomainInvitePreview";
import { useTitle } from "@/hooks/useTitle";
import domainStore from "@/stores/domainStore";
import userStore from "@/stores/userStore";
import { extractDomainInviteCode } from "@/utils/domainInvite";

export const Route = createFileRoute("/invite/d/$code/")({
	beforeLoad: async () => {
		// 邀请页独立于 (app) 壳，自行保证会话；未登录时登录后回跳本页
		const ok = await userStore.ensureSession();
		if (!ok) {
			if (typeof window !== "undefined") {
				sessionStorage.setItem(
					"gospeak_redirect",
					window.location.pathname + window.location.search,
				);
			}
			throw redirect({ to: "/login" });
		}
	},
	component: RouteComponent,
	staticData: { title: "邀请加入" },
});

function RouteComponent() {
	useTitle();
	const params = Route.useParams();
	const navigate = useNavigate();
	const inviteCode = createMemo(() => extractDomainInviteCode(params().code));
	const [domain] = createResource(inviteCode, (code) =>
		previewDomainInvite(code),
	);
	const [joining, setJoining] = createSignal(false);
	const [error, setError] = createSignal("");
	// standalone 页没有应用壳代为加载 myDomains，需自行拉取才能判断是否已加入
	const [membershipReady, setMembershipReady] = createSignal(false);
	onMount(() => {
		domainStore
			.loadMyDomains()
			.catch(() => {})
			.finally(() => setMembershipReady(true));
	});

	// fetcher 失败后读取 resource() 会重新抛错，需先检查 error 再取值
	const safeDomain = () => (domain.error ? undefined : domain());

	const joined = createMemo(() => {
		const current = safeDomain();
		return !!current && domainStore.state.myDomainUUIDs.includes(current.uuid);
	});

	const status = () =>
		getDomainInvitePreviewStatus(
			!!domain.loading,
			domain.error ? "error" : null,
			safeDomain(),
		);

	const action = () =>
		getDomainInviteAction(safeDomain(), joined(), joining(), handleJoin);

	async function handleJoin() {
		const current = safeDomain();
		if (!current) return;
		if (joining()) return;
		if (joined()) {
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: current.uuid },
			});
			return;
		}
		setJoining(true);
		setError("");
		try {
			const joinedDomain = await joinDomain(current.invite_code);
			domainStore.addDomain(joinedDomain);
			domainStore.setCurrentDomain(joinedDomain.uuid);
			navigate({
				to: "/domain/$domainUUID",
				params: { domainUUID: joinedDomain.uuid },
			});
		} catch (e: any) {
			const msg: string = e?.response?.data?.msg || "";
			if (e?.response?.data?.code === 3002 || /already a member/i.test(msg)) {
				// 服务端判定已是成员：刷新本地域列表，按钮翻转为「进入域」
				await domainStore.loadMyDomains().catch(() => {});
				setError("");
				return;
			}
			setError(msg || "加入失败");
		} finally {
			setJoining(false);
		}
	}

	return (
		<div class="invite-shell">
			<style>
				{`.invite-shell {
	position: relative;
	min-height: 100vh;
	overflow-y: auto;
	background: var(--b2);
}
.invite-shell::before {
	content: "";
	position: fixed;
	inset: -60px;
	background-image:
		linear-gradient(to right, rgb(127 127 127 / 0.07) 1px, transparent 1px),
		linear-gradient(to bottom, rgb(127 127 127 / 0.07) 1px, transparent 1px);
	background-size: 44px 44px;
	mask-image: radial-gradient(ellipse 90% 80% at 50% 45%, black 20%, transparent 80%);
	pointer-events: none;
}`}
			</style>
			<div class="relative flex min-h-screen w-full flex-col items-center justify-center px-4 py-12">
				<header class="mb-10 flex items-center gap-3">
					<img
						src="/favicon-256.png"
						alt="GOSpeak"
						class="size-10 rounded-xl border border-base-content/10"
					/>
					<span class="text-lg font-extrabold uppercase tracking-wide">
						GO<span class="text-primary">Speak</span>
					</span>
				</header>

				<Switch>
					<Match when={!inviteCode() || status() === "error"}>
						<InvalidInviteCard onHome={() => navigate({ to: "/" })} />
					</Match>
					<Match when={status() === "loading"}>
						<div class="card w-full max-w-sm border border-base-content/10 bg-base-100 shadow-xl shadow-black/10">
							<div class="card-body items-center gap-4 p-8">
								<div class="skeleton size-16 rounded-2xl" />
								<div class="skeleton h-5 w-36" />
								<div class="skeleton h-3 w-52" />
								<div class="skeleton h-11 w-full" />
							</div>
						</div>
					</Match>
					<Match when={safeDomain()}>
						{(d) => (
							<div class="card w-full max-w-sm border border-base-content/10 bg-base-100 shadow-xl shadow-black/10">
								<div class="card-body items-center gap-2 p-8 text-center">
									<p class="font-mono text-[11px] uppercase tracking-[0.25em] text-base-content/40">
										Invite
									</p>
									<h1 class="mb-3 text-base font-medium text-base-content/70">
										你被邀请加入语音服务器
									</h1>
									<Show
										when={d().icon_url}
										fallback={
											<div class="flex size-16 items-center justify-center rounded-2xl bg-primary/15 text-xl font-bold text-primary">
												{d().name.slice(0, 2).toUpperCase()}
											</div>
										}
									>
										<img
											src={d().icon_url}
											alt={d().name}
											class="size-16 rounded-2xl border border-base-content/10 object-cover"
										/>
									</Show>
									<h2 class="mt-2 text-xl font-semibold">{d().name}</h2>
									<Show when={d().description}>
										<p class="text-sm text-base-content/60">
											{d().description}
										</p>
									</Show>
									<span class="badge badge-outline badge-sm mt-1 text-base-content/50">
										{d().is_public ? "公开" : "私有"}
									</span>
									<Show
										when={membershipReady()}
										fallback={<div class="skeleton mt-4 h-11 w-full" />}
									>
										<Show when={error()}>
											<div role="alert" class="alert alert-error mt-2 text-sm">
												<span>{error()}</span>
											</div>
										</Show>
										<Show when={joined()}>
											<p class="mt-2 text-xs text-base-content/50">
												你已经是该语音服务器的成员
											</p>
										</Show>
										<button
											type="button"
											class="btn btn-primary mt-4 w-full"
											disabled={action()?.disabled}
											onClick={() => action()?.onClick()}
										>
											<Show when={joining()}>
												<span class="loading loading-spinner loading-xs" />
											</Show>
											{action()?.label}
										</button>
									</Show>
								</div>
							</div>
						)}
					</Match>
				</Switch>

				<p class="mt-10 text-xs text-base-content/40">
					GOSpeak · 自托管游戏语音平台
				</p>
			</div>
		</div>
	);
}

function InvalidInviteCard(props: { onHome: () => void }) {
	return (
		<div class="card w-full max-w-sm border border-base-content/10 bg-base-100 shadow-xl shadow-black/10">
			<div class="card-body items-center gap-3 p-8 text-center">
				<div class="flex size-14 items-center justify-center rounded-2xl bg-error/10 text-error">
					<Link2Off size={26} />
				</div>
				<h1 class="text-lg font-semibold">邀请链接无效或已失效</h1>
				<p class="text-sm text-base-content/60">
					链接可能已被重置或输入有误，请联系邀请人获取新的邀请链接。
				</p>
				<button type="button" class="btn btn-ghost mt-2" onClick={props.onHome}>
					返回首页
				</button>
			</div>
		</div>
	);
}
