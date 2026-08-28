import {
	Link,
	createFileRoute,
	redirect,
	useNavigate,
} from "@tanstack/solid-router";
import { createSignal, onCleanup, Show } from "solid-js";
import { showToast } from "solid-notifications";
import { register as registerApi } from "@/api/auth";
import { sendEmailCode } from "@/api/email";
import userStore from "@/stores/userStore";

export const Route = createFileRoute("/register/")({
	beforeLoad: async () => {
		// 已有会话或 access 过期但仍可无感刷新时，直接进首页
		const ok = await userStore.ensureSession();
		if (ok) {
			throw redirect({ to: "/" });
		}
	},
	component: RegisterPage,
});

function resolveRegisterRedirect(): string {
	const redirectTo = sessionStorage.getItem("gospeak_redirect");
	sessionStorage.removeItem("gospeak_redirect");
	return redirectTo?.startsWith("/") ? redirectTo : "/";
}

function errCode(e: unknown): number {
	return (
		(e as { response?: { data?: { code?: number } } })?.response?.data?.code ??
		0
	);
}

function errMessage(e: unknown): string {
	return (
		(e as { response?: { data?: { msg?: string } } })?.response?.data?.msg ?? ""
	);
}

function registerErrorMessage(code: number, msg: string): string {
	if (msg.includes("guest_")) return "用户名不能以 guest_ 开头";
	if (msg.includes("bot_")) return "用户名不能以 bot_ 开头";
	switch (code) {
		case 1012:
			return "用户名已存在";
		case 1017:
			return "请求过于频繁，请稍后再试";
		case 8001:
			return "该邮箱已被注册";
		case 8002:
		case 8003:
		case 8004:
		case 8008:
			return "验证码无效或已过期";
		case 2001:
			if (msg.includes("email")) {
				return "该服务器已开启邮箱验证，请填写邮箱与验证码";
			}
			return msg || "参数有误";
		default:
			return msg || "注册失败，请稍后再试";
	}
}

function RegisterPage() {
	const navigate = useNavigate();

	let shellRef!: HTMLDivElement;

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

	const [username, setUsername] = createSignal("");
	const [password, setPassword] = createSignal("");
	const [confirmPassword, setConfirmPassword] = createSignal("");
	const [email, setEmail] = createSignal("");
	const [emailCode, setEmailCode] = createSignal("");
	const [submitting, setSubmitting] = createSignal(false);
	const [errorMsg, setErrorMsg] = createSignal("");
	const [touched, setTouched] = createSignal({
		username: false,
		password: false,
		confirm: false,
	});

	const [codeSending, setCodeSending] = createSignal(false);
	const [codeCountdown, setCodeCountdown] = createSignal(0);
	let countdownTimer: ReturnType<typeof setInterval> | undefined;
	onCleanup(() => clearInterval(countdownTimer));

	const usernameError = () => {
		if (!touched().username) return "";
		if (!username().trim()) return "用户名 是必填项";
		return "";
	};
	const passwordError = () => {
		if (!touched().password) return "";
		if (!password()) return "密码 是必填项";
		if (password().length < 8) return "密码至少 8 位";
		return "";
	};
	const confirmPasswordError = () => {
		if (!touched().confirm) return "";
		if (!confirmPassword()) return "确认密码 是必填项";
		if (confirmPassword() !== password()) return "两次输入的密码不一致";
		return "";
	};

	async function handleSendCode() {
		if (codeSending() || codeCountdown() > 0 || !email().trim()) return;
		setCodeSending(true);
		try {
			const res = await sendEmailCode({
				email: email().trim(),
				scene: "register",
			});
			showToast("验证码已发送，请查收邮箱", { type: "success" });
			setCodeCountdown(res.expires_in || 60);
			countdownTimer = setInterval(() => {
				setCodeCountdown((n) => {
					if (n <= 1) clearInterval(countdownTimer);
					return n <= 1 ? 0 : n - 1;
				});
			}, 1000);
		} catch (e) {
			showToast(registerErrorMessage(errCode(e), errMessage(e)), {
				type: "error",
			});
		} finally {
			setCodeSending(false);
		}
	}

	async function handleRegisterSubmit(e?: Event) {
		e?.preventDefault();
		e?.stopPropagation();
		setTouched({ username: true, password: true, confirm: true });
		if (
			!username().trim() ||
			!password() ||
			password().length < 8 ||
			!confirmPassword() ||
			confirmPassword() !== password()
		) {
			return;
		}
		if (submitting()) return;
		setSubmitting(true);
		setErrorMsg("");
		try {
			const data = await registerApi({
				username: username().trim(),
				password: password(),
				email: email().trim(),
				email_code: emailCode().trim(),
			});
			await userStore.login(data.user, data.expires_in);
			showToast("注册成功", { type: "success" });
			navigate({ to: resolveRegisterRedirect() });
		} catch (err) {
			setErrorMsg(registerErrorMessage(errCode(err), errMessage(err)));
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
							<p class="mt-2 text-sm text-base-content/50">创建你的账号</p>
						</div>
						<div class="card-tilt">
							<div class="card border border-base-content/10 bg-base-100 shadow-xl shadow-black/20">
								<div class="card-body p-6 sm:p-8">
									<div class="mb-6 hidden items-center justify-between md:flex">
										<h2 class="text-lg font-semibold">注册账号</h2>
										<span class="font-mono text-[11px] uppercase text-base-content/40">
											Register
										</span>
									</div>

									<form onSubmit={handleRegisterSubmit} noValidate>
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
												placeholder="至少 8 位"
												required
												autocomplete="new-password"
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
										<fieldset class="fieldset relative mb-4">
											<legend class="fieldset-legend text-[14px]">
												确认密码
											</legend>
											<input
												id="field-confirm-password"
												type="password"
												value={confirmPassword()}
												placeholder="再次输入密码"
												required
												autocomplete="new-password"
												class="input w-full"
												onInput={(e) =>
													setConfirmPassword(e.currentTarget.value)
												}
												onBlur={() =>
													setTouched((p) => ({ ...p, confirm: true }))
												}
											/>
											<Show when={confirmPasswordError()}>
												<p class="absolute top-full left-1 mt-1 text-xs text-error">
													{confirmPasswordError()}
												</p>
											</Show>
										</fieldset>
										<fieldset class="fieldset relative mb-4">
											<legend class="fieldset-legend text-[14px]">邮箱</legend>
											<input
												id="field-email"
												type="email"
												value={email()}
												placeholder="请输入邮箱"
												autocomplete="email"
												class="input w-full"
												onInput={(e) => setEmail(e.currentTarget.value)}
											/>
											<p class="mt-1 text-xs text-base-content/50">
												选填。若服务器开启邮箱验证，则邮箱与验证码必填
											</p>
										</fieldset>
										<Show when={email().trim()}>
											<fieldset class="fieldset relative mb-4">
												<legend class="fieldset-legend text-[14px]">
													验证码
												</legend>
												<div class="flex items-center gap-2">
													<input
														id="field-email-code"
														type="text"
														value={emailCode()}
														placeholder="请输入验证码"
														autocomplete="one-time-code"
														class="input w-full"
														onInput={(e) => setEmailCode(e.currentTarget.value)}
													/>
													<button
														type="button"
														class="btn btn-outline shrink-0"
														disabled={
															codeSending() ||
															codeCountdown() > 0 ||
															!email().trim()
														}
														onClick={handleSendCode}
													>
														{codeSending()
															? "发送中..."
															: codeCountdown() > 0
																? `${codeCountdown()}s 后重发`
																: "发送验证码"}
													</button>
												</div>
											</fieldset>
										</Show>

										<Show when={errorMsg()}>
											<div
												role="alert"
												class="alert alert-error mb-2 py-2 text-sm"
											>
												<span>{errorMsg()}</span>
											</div>
										</Show>

										<button
											type="submit"
											class="btn btn-primary mt-4 w-full"
											disabled={submitting()}
										>
											{submitting() ? "注册中..." : "注册"}
										</button>
									</form>

									<div class="text-center mt-1">
										<Link
											to="/login"
											class="link link-primary text-sm inline-flex items-center justify-center px-2 min-h-11"
										>
											已有账号？去登录
										</Link>
									</div>
								</div>
							</div>
						</div>
					</div>
				</main>
			</div>
		</div>
	);
}
