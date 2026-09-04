import {
	Link,
	createFileRoute,
	redirect,
	useNavigate,
} from "@tanstack/solid-router";
import {
	createResource,
	createSignal,
	For,
	onCleanup,
	onMount,
	Show,
} from "solid-js";
import { showToast } from "solid-notifications";
import CircleX from "lucide-solid/icons/circle-x";
import {
	firstChangePassword as firstChangePasswordApi,
	login as loginApi,
} from "@/api/auth";
import { getEnabledProviders, getOAuthLoginURL } from "@/api/oauth";
import ProviderIcon from "@/components/oauth/ProviderIcon";
import userStore from "@/stores/userStore";
import ForcePasswordChangeModal from "./components/ForcePasswordChangeModal";
import ForgotPasswordModal from "./components/ForgotPasswordModal";
import { showLoginSuccessToast } from "./components/loginSuccessToast";

export const Route = createFileRoute("/login/")({
	beforeLoad: async () => {
		// 已有会话或 access 过期但仍可无感刷新时，直接进首页
		const ok = await userStore.ensureSession();
		if (ok) {
			throw redirect({ to: "/" });
		}
	},
	component: LoginPage,
});

function resolveLoginRedirect(): string {
	const redirectTo = sessionStorage.getItem("gospeak_redirect");
	sessionStorage.removeItem("gospeak_redirect");
	return redirectTo?.startsWith("/") ? redirectTo : "/";
}

function LoginPage() {
	const navigate = useNavigate();
	const [banned, setBanned] = createSignal(false);
	const [showForgotModal, setShowForgotModal] = createSignal(false);
	const [forgotEmail, setForgotEmail] = createSignal("");
	const [forgotCode, setForgotCode] = createSignal("");
	const [forgotStep, setForgotStep] = createSignal<"email" | "code">("email");
	const [codeSending, setCodeSending] = createSignal(false);

	const [oauthProviders] = createResource(getEnabledProviders);
	const [oauthLoading, setOauthLoading] = createSignal(false);

	let shellRef!: HTMLDivElement;

	function completeOAuthLogin(expiresIn?: number) {
		void (async () => {
			setOauthLoading(true);
			try {
				// token 已由服务端写入 HttpOnly Cookie，这里先记录过期时间再确认会话并拉取 profile。
				userStore.recordSessionExpiry(expiresIn);
				const ok = await userStore.ensureSession();
				if (!ok) {
					showToast("OAuth 登录失败：未能建立会话", { type: "error" });
					return;
				}
				showLoginSuccessToast();
				navigate({ to: resolveLoginRedirect() });
			} catch (e) {
				await userStore.clearAuth();
				const code = (e as { response?: { data?: { code?: number } } })
					?.response?.data?.code;
				if (code === 1015) {
					setBanned(true);
				}
			} finally {
				setOauthLoading(false);
			}
		})();
	}

	function handleOAuthMessage(event: MessageEvent) {
		if (event.origin !== window.location.origin) return;
		const data = event.data as {
			type?: string;
			ok?: boolean;
			expires_in?: number;
		} | null;
		if (data?.type !== "gospeak-oauth") return;
		if (data.ok) completeOAuthLogin(data.expires_in);
	}

	onMount(() => {
		const params = new URLSearchParams(window.location.search);
		if (params.get("banned") === "1") {
			setBanned(true);
			window.history.replaceState({}, "", "/login");
			return;
		}

		const oauthError = params.get("oauth_error");
		if (oauthError) {
			showToast(oauthError, { type: "error" });
			window.history.replaceState({}, "", "/login");
			return;
		}

		window.addEventListener("message", handleOAuthMessage);
	});

	onCleanup(() => {
		window.removeEventListener("message", handleOAuthMessage);
	});

	// 鼠标视差：目标点做 lerp 缓动，写入 --px/--py，各层按 --depth 取位移
	const reducedMotion =
		typeof window !== "undefined" &&
		window.matchMedia("(prefers-reduced-motion: reduce)").matches;

	if (!reducedMotion) {
		let targetX = 0;
		let targetY = 0;
		let curX = 0;
		let curY = 0;
		let raf = 0;
		const onMove = (e: MouseEvent) => {
			targetX = e.clientX / window.innerWidth - 0.5;
			targetY = e.clientY / window.innerHeight - 0.5;
		};
		const tick = () => {
			curX += (targetX - curX) * 0.08;
			curY += (targetY - curY) * 0.08;
			if (shellRef) {
				shellRef.style.setProperty("--px", curX.toFixed(4));
				shellRef.style.setProperty("--py", curY.toFixed(4));
			}
			raf = requestAnimationFrame(tick);
		};
		window.addEventListener("mousemove", onMove);
		raf = requestAnimationFrame(tick);
		onCleanup(() => {
			window.removeEventListener("mousemove", onMove);
			cancelAnimationFrame(raf);
		});
	}

	const [showChangeModal, setShowChangeModal] = createSignal(false);

	let forgotDialogRef!: HTMLDialogElement;
	let changeDialogRef!: HTMLDialogElement;

	const openForgotModal = () => {
		setShowForgotModal(true);
		forgotDialogRef?.showModal();
	};

	const closeForgotModal = () => {
		forgotDialogRef?.close();
		setShowForgotModal(false);
	};

	const openChangeModal = () => {
		setShowChangeModal(true);
		changeDialogRef?.showModal();
	};

	const closeChangeModal = () => {
		changeDialogRef?.close();
		setShowChangeModal(false);
	};

	const [username, setUsername] = createSignal("");
	const [password, setPassword] = createSignal("");
	const [submitting, setSubmitting] = createSignal(false);
	const [touched, setTouched] = createSignal({
		username: false,
		password: false,
	});

	const usernameError = () => {
		if (!touched().username) return "";
		if (!username().trim()) return "用户名 是必填项";
		return "";
	};
	const passwordError = () => {
		if (!touched().password) return "";
		if (!password()) return "密码 是必填项";
		return "";
	};

	async function handleLoginSubmit(e?: Event) {
		e?.preventDefault();
		e?.stopPropagation();
		setTouched({ username: true, password: true });
		if (!username().trim() || !password()) {
			return;
		}
		if (submitting()) return;
		setSubmitting(true);
		try {
			const data = await loginApi({
				username: username().trim(),
				password: password(),
			});
			if (data.need_change_password) {
				await userStore.login(data.user, data.expires_in);
				openChangeModal();
				return;
			}
			await userStore.login(data.user, data.expires_in);
			showLoginSuccessToast();
			navigate({ to: resolveLoginRedirect() });
		} catch (err: any) {
			if (err?.response?.data?.code === 1015) {
				setBanned(true);
			}
		} finally {
			setSubmitting(false);
		}
	}

	const depth = (n: number) =>
		({ "--depth": String(n) }) as unknown as Record<string, string>;

	return (
		<div ref={shellRef} class="login-shell">
			<style>
				{`				.login-shell {
					--px: 0;
					--py: 0;
					position: relative;
					min-height: 100vh;
					overflow-y: auto;
					background: var(--b2);
				}
				/* 网格纹理：中心清晰、四周淡出，鼠标移动时反向漂移 */
				.login-shell::before {
					content: "";
					position: fixed;
					inset: -60px;
					background-image:
						linear-gradient(to right, rgb(127 127 127 / 0.07) 1px, transparent 1px),
						linear-gradient(to bottom, rgb(127 127 127 / 0.07) 1px, transparent 1px);
					background-size: 44px 44px;
					mask-image: radial-gradient(ellipse 90% 80% at 50% 45%, black 20%, transparent 80%);
					transform: translate3d(calc(var(--px) * -22px), calc(var(--py) * -22px), 0);
					pointer-events: none;
				}
				/* 视差层：位移 = 鼠标偏移 × 深度系数 */
				.plx {
					transform: translate3d(
						calc(var(--px) * var(--depth, 0) * 1px),
						calc(var(--py) * var(--depth, 0) * 1px),
						0
					);
					will-change: transform;
				}
				/* 登录卡片：3D 微倾 + 轻微位移 */
				.card-tilt {
					transform: perspective(1100px)
						rotateX(calc(var(--py) * -4deg))
						rotateY(calc(var(--px) * 4deg))
						translate3d(calc(var(--px) * 8px), calc(var(--py) * 8px), 0);
					will-change: transform;
					transform-style: preserve-3d;
				}
				.brand-word {
					font-size: 3.5rem;
					line-height: 1;
					font-weight: 800;
					text-transform: uppercase;
				}
				@media (min-width: 1024px) {
					.brand-word {
						font-size: 4.5rem;
					}
				}
				.brand-word em {
					font-style: normal;
					color: var(--p);
				}
				/* 均衡器：7 根音柱错相起伏 */
				.eq {
					display: flex;
					align-items: center;
					height: 40px;
					gap: 5px;
				}
				.eq span {
					width: 4px;
					height: 100%;
					border-radius: 2px;
					background: var(--p);
					transform-origin: center;
					animation: login-eq 1.2s ease-in-out infinite;
				}
				.eq span:nth-child(1) { animation-delay: 0s; opacity: 0.55; }
				.eq span:nth-child(2) { animation-delay: -0.15s; opacity: 0.75; }
				.eq span:nth-child(3) { animation-delay: -0.3s; opacity: 0.95; }
				.eq span:nth-child(4) { animation-delay: -0.45s; opacity: 1; }
				.eq span:nth-child(5) { animation-delay: -0.6s; opacity: 0.9; }
				.eq span:nth-child(6) { animation-delay: -0.75s; opacity: 0.7; }
				.eq span:nth-child(7) { animation-delay: -0.9s; opacity: 0.5; }
				@keyframes login-eq {
					0%, 100% { transform: scaleY(0.25); }
					50% { transform: scaleY(1); }
				}
				@media (prefers-reduced-motion: reduce) {
					.eq span {
						animation: none;
						transform: scaleY(0.6);
					}
					.plx,
					.card-tilt,
					.login-shell::before {
						transform: none;
					}
				}`}
			</style>
			<div class="relative mx-auto flex min-h-screen w-full max-w-7xl">
				{/* 品牌区：桌面端左半屏，移动端只显示 logo 头部 */}
				<aside class="relative hidden w-1/2 flex-col justify-between gap-12 border-r border-base-content/10 p-12 md:flex lg:p-16">
					<div class="plx flex items-center gap-3" style={depth(10)}>
						<img
							src="/favicon-256.png"
							alt="GOSpeak"
							class="size-10 rounded-lg border border-base-content/10"
						/>
						<span class="font-mono text-xs uppercase text-base-content/50">
							Self-hosted Voice
						</span>
					</div>
					<div>
						<h1 class="brand-word plx" style={depth(16)}>
							GO<em>Speak</em>
						</h1>
						<p class="mt-4 text-base text-base-content/60 plx" style={depth(8)}>
							自托管游戏语音平台。房间、路由与数据，全部在你手里。
						</p>
						<div class="eq mt-10 plx" style={depth(22)} aria-hidden="true">
							<span />
							<span />
							<span />
							<span />
							<span />
							<span />
							<span />
						</div>
						<ul
							class="mt-10 space-y-2 text-sm text-base-content/50 plx"
							style={depth(6)}
						>
							<li>实时发言检测 · 成员独立音量</li>
							<li>多 SFU 运行时切换</li>
							<li>语音数据不经第三方</li>
						</ul>
					</div>
					<div
						class="plx flex items-center gap-2 font-mono text-xs text-base-content/40"
						style={depth(4)}
					>
						<span class="size-1.5 rounded-full bg-primary animate-pulse" />
						Server Online
					</div>
				</aside>
				<main class="flex flex-1 items-center justify-center p-4 sm:p-8">
					<div class="w-full max-w-sm">
						<div class="mb-8 text-center plx md:hidden" style={depth(10)}>
							<div class="flex items-center justify-center gap-3">
								<img
									src="/favicon-256.png"
									alt="GOSpeak"
									class="size-12 rounded-xl border border-base-content/10"
								/>
								<h1 class="text-2xl font-extrabold uppercase">
									GO<span class="text-primary">Speak</span>
								</h1>
							</div>
							<p class="mt-2 text-sm text-base-content/50">登录你的账号</p>
						</div>
						<div class="card-tilt">
							<div class="card border border-base-content/10 bg-base-100 shadow-xl shadow-black/20">
								<div class="card-body p-6 sm:p-8">
									<div class="mb-6 hidden items-center justify-between md:flex">
										<h2 class="text-lg font-semibold">登录账号</h2>
										<span class="font-mono text-[11px] uppercase text-base-content/40">
											Login
										</span>
									</div>

									<form onSubmit={handleLoginSubmit} noValidate>
										<fieldset class="fieldset relative mb-4">
											<legend class="fieldset-legend text-[14px]">
												用户名
											</legend>
											<input
												id="field-username"
												type="text"
												value={username()}
												placeholder="请输入用户名"
												required
												autocomplete="username"
												class="input w-full"
												onInput={(e) => setUsername(e.currentTarget.value)}
												onBlur={() =>
													setTouched((p) => ({ ...p, username: true }))
												}
											/>
											<Show when={usernameError()}>
												<p class="absolute top-full left-1 mt-1 text-xs text-error">
													{usernameError()}
												</p>
											</Show>
										</fieldset>
										<fieldset class="fieldset relative mb-4">
											<legend class="fieldset-legend text-[14px]">密码</legend>
											<input
												id="field-password"
												type="password"
												value={password()}
												placeholder="请输入密码"
												required
												autocomplete="current-password"
												class="input w-full"
												onInput={(e) => setPassword(e.currentTarget.value)}
												onBlur={() =>
													setTouched((p) => ({ ...p, password: true }))
												}
											/>
											<Show when={passwordError()}>
												<p class="absolute top-full left-1 mt-1 text-xs text-error">
													{passwordError()}
												</p>
											</Show>
										</fieldset>
										<button
											type="submit"
											class="btn btn-primary mt-4 w-full"
											disabled={submitting()}
										>
											{submitting() ? "登录中..." : "登录"}
										</button>
									</form>

									<div class="mt-1 flex items-center justify-center">
										<button
											type="button"
											class="link link-primary text-sm inline-flex items-center justify-center px-2 min-h-11"
											onClick={openForgotModal}
										>
											忘记密码?
										</button>
										<span class="text-base-content/20">·</span>
										<Link
											to="/register"
											class="link link-primary text-sm inline-flex items-center justify-center px-2 min-h-11"
										>
											注册账号
										</Link>
									</div>

									{/* 已启用的第三方登录：按提供商显示品牌 icon 按钮 */}
									<Show
										when={
											!oauthProviders.error &&
											(oauthProviders()?.length || 0) > 0
										}
									>
										<div class="divider text-xs text-base-content/40 my-3">
											或使用第三方登录
										</div>
										<div class="flex flex-wrap items-center justify-center gap-3">
											<For each={oauthProviders() ?? []}>
												{(p) => {
													const label = p.display_name || p.name;
													return (
														<button
															type="button"
															class="btn btn-outline btn-square size-12"
															title={`使用 ${label} 登录`}
															aria-label={`使用 ${label} 登录`}
															disabled={oauthLoading()}
															onClick={() => {
																setOauthLoading(true);
																const win = window.open(
																	getOAuthLoginURL(p.name),
																	"gospeak-oauth",
																	"popup,width=560,height=640",
																);
																if (!win) {
																	setOauthLoading(false);
																	showToast("请允许弹窗后重试 OAuth 登录", {
																		type: "error",
																	});
																}
															}}
														>
															<ProviderIcon
																name={p.name}
																iconUrl={p.icon_url}
																size={22}
															/>
														</button>
													);
												}}
											</For>
										</div>
									</Show>

									<Show when={oauthLoading()}>
										<div class="flex items-center justify-center gap-2 mt-3 text-sm text-base-content/60">
											<span class="loading loading-spinner loading-xs" />
											正在完成第三方登录…
										</div>
									</Show>

									<Show when={banned()}>
										<div class="alert alert-error mt-2">
											<CircleX class="shrink-0 size-5" />
											<span class="text-sm">
												您的账号已被封禁，无法登录。如有疑问请联系管理员。
											</span>
										</div>
									</Show>
								</div>
							</div>
						</div>
					</div>
				</main>
			</div>

			{/* 忘记密码 Modal */}
			<ForgotPasswordModal
				open={showForgotModal()}
				email={forgotEmail()}
				code={forgotCode()}
				step={forgotStep()}
				codeSending={codeSending()}
				dialogRef={(el) => {
					forgotDialogRef = el;
				}}
				onClose={closeForgotModal}
				setEmail={setForgotEmail}
				setCode={setForgotCode}
				setStep={setForgotStep}
				setCodeSending={setCodeSending}
			/>

			{/* Admin 首次登录强制改密 Modal */}
			<ForcePasswordChangeModal
				open={showChangeModal()}
				dialogRef={(el) => {
					changeDialogRef = el;
				}}
				onClose={closeChangeModal}
				onSubmit={async ({ newPassword, name }) => {
					const result = await firstChangePasswordApi(newPassword ?? "", name);
					await userStore.login(result.user, result.expires_in);
					closeChangeModal();
					showLoginSuccessToast();
					navigate({ to: resolveLoginRedirect() });
				}}
			/>
		</div>
	);
}
