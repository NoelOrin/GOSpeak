import { Button, Input, toast } from "@gospeak/components";
import {
	Link,
	createFileRoute,
	redirect,
	useNavigate,
} from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { register as registerApi } from "@/api/auth";
import { sendEmailCode } from "@/api/email";
import PasswordInput from "@/components/common/passwordInput";
import { useUserStore } from "@/stores/userStore";
import { depth, useAuthParallax } from "@/hooks/useAuthParallax";

export const Route = createFileRoute("/register")({
	beforeLoad: async () => {
		// 已有会话或 access 过期但仍可无感刷新时，直接进首页
		const ok = await useUserStore.getState().ensureSession();
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
	const shellRef = useRef<HTMLDivElement>(null);
	useAuthParallax(shellRef);

	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [confirmPassword, setConfirmPassword] = useState("");
	const [email, setEmail] = useState("");
	const [emailCode, setEmailCode] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [errorMsg, setErrorMsg] = useState("");
	const [touched, setTouched] = useState({
		username: false,
		password: false,
		confirm: false,
	});

	const [codeSending, setCodeSending] = useState(false);
	const [codeCountdown, setCodeCountdown] = useState(0);
	const countdownTimer = useRef<ReturnType<typeof setInterval> | undefined>(
		undefined,
	);
	useEffect(() => () => clearInterval(countdownTimer.current), []);

	const usernameError =
		touched.username && !username.trim() ? "用户名 是必填项" : "";
	const passwordError =
		touched.password && !password
			? "密码 是必填项"
			: touched.password && password.length < 8
				? "密码至少 8 位"
				: "";
	const confirmPasswordError =
		touched.confirm && !confirmPassword
			? "确认密码 是必填项"
			: touched.confirm && confirmPassword !== password
				? "两次输入的密码不一致"
				: "";

	async function handleSendCode() {
		if (codeSending || codeCountdown > 0 || !email.trim()) return;
		setCodeSending(true);
		try {
			const res = await sendEmailCode(
				{
					email: email.trim(),
					scene: "register",
				},
				// 失败文案按业务码定制，拦截器不再重复弹
				{ skipErrorToast: true },
			);
			toast.success("验证码已发送，请查收邮箱");
			setCodeCountdown(res.expires_in || 60);
			countdownTimer.current = setInterval(() => {
				setCodeCountdown((n) => {
					if (n <= 1) clearInterval(countdownTimer.current);
					return n <= 1 ? 0 : n - 1;
				});
			}, 1000);
		} catch (e) {
			toast.error(registerErrorMessage(errCode(e), errMessage(e)));
		} finally {
			setCodeSending(false);
		}
	}

	async function handleRegisterSubmit(e?: React.FormEvent) {
		e?.preventDefault();
		e?.stopPropagation();
		setTouched({ username: true, password: true, confirm: true });
		if (
			!username.trim() ||
			!password ||
			password.length < 8 ||
			!confirmPassword ||
			confirmPassword !== password
		) {
			return;
		}
		if (submitting) return;
		setSubmitting(true);
		setErrorMsg("");
		try {
			const data = await registerApi({
				username: username.trim(),
				password,
				email: email.trim(),
				email_code: emailCode.trim(),
			});
			await useUserStore.getState().login(data.user, data.expires_in);
			toast.success("注册成功");
			navigate({ to: resolveRegisterRedirect() });
		} catch (err) {
			setErrorMsg(registerErrorMessage(errCode(err), errMessage(err)));
		} finally {
			setSubmitting(false);
		}
	}

	return (
		<div ref={shellRef} className="login-shell">
			<div className="relative mx-auto flex w-full max-w-7xl flex-1">
				{/* 品牌区：桌面端左半屏，移动端只显示 logo 头部 */}
				<aside className="border-foreground/10 relative hidden w-1/2 flex-col justify-between gap-12 border-r p-12 md:flex lg:p-16">
					<div className="plx flex items-center gap-3" style={depth(10)}>
						<img
							src="/favicon-256.png"
							alt="GOSpeak"
							className="border-foreground/10 size-10 rounded-lg border"
						/>
						<span className="text-faint-foreground font-mono text-xs uppercase">
							Self-hosted Voice
						</span>
					</div>
					<div>
						<h1 className="brand-word plx" style={depth(16)}>
							GO<em>Speak</em>
						</h1>
						<p
							className="text-muted-foreground mt-4 text-base plx"
							style={depth(8)}
						>
							自托管游戏语音平台。房间、路由与数据，全部在你手里。
						</p>
						<div className="eq mt-10 plx" style={depth(22)} aria-hidden="true">
							<span />
							<span />
							<span />
							<span />
							<span />
							<span />
							<span />
						</div>
						<ul
							className="text-faint-foreground mt-10 space-y-2 text-sm plx"
							style={depth(6)}
						>
							<li>实时发言检测 · 成员独立音量</li>
							<li>多 SFU 运行时切换</li>
							<li>语音数据不经第三方</li>
						</ul>
					</div>
					<div
						className="text-faint-foreground plx flex items-center gap-2 font-mono text-xs"
						style={depth(4)}
					>
						<span className="bg-primary size-1.5 animate-pulse rounded-full" />
						Server Online
					</div>
				</aside>
				<main className="flex flex-1 items-center justify-center p-4 sm:p-8">
					<div className="w-full max-w-sm">
						<div className="plx mb-8 text-center md:hidden" style={depth(10)}>
							<div className="flex items-center justify-center gap-3">
								<img
									src="/favicon-256.png"
									alt="GOSpeak"
									className="border-foreground/10 size-12 rounded-lg border"
								/>
								<h1 className="text-2xl font-extrabold uppercase">
									GO<span className="text-primary">Speak</span>
								</h1>
							</div>
							<p className="text-faint-foreground mt-2 text-sm">创建你的账号</p>
						</div>
						<div className="card-tilt">
							<div className="border-foreground/10 bg-card rounded-lg border shadow-xl shadow-black/20">
								<div className="p-6 sm:p-8">
									<div className="mb-6 hidden items-center justify-between md:flex">
										<h2 className="text-lg font-semibold">注册账号</h2>
										<span className="text-faint-foreground font-mono text-2xs uppercase">
											Register
										</span>
									</div>

									<form onSubmit={handleRegisterSubmit} noValidate>
										<label className="relative mb-4 block">
											<span className="mb-1 block text-sm">用户名</span>
											<Input
												type="text"
												value={username}
												placeholder="请输入用户名"
												required
												autoComplete="username"
												onChange={(e) => setUsername(e.target.value)}
												onBlur={() =>
													setTouched((p) => ({ ...p, username: true }))
												}
											/>
											{usernameError && (
												<p className="text-destructive absolute top-full left-1 mt-1 text-xs">
													{usernameError}
												</p>
											)}
										</label>
										<label className="relative mb-4 block">
											<span className="mb-1 block text-sm">密码</span>
											<PasswordInput
												value={password}
												placeholder="至少 8 位"
												required
												autoComplete="new-password"
												onChange={(e) => setPassword(e.target.value)}
												onBlur={() =>
													setTouched((p) => ({ ...p, password: true }))
												}
											/>
											{passwordError && (
												<p className="text-destructive absolute top-full left-1 mt-1 text-xs">
													{passwordError}
												</p>
											)}
										</label>
										<label className="relative mb-4 block">
											<span className="mb-1 block text-sm">确认密码</span>
											<PasswordInput
												value={confirmPassword}
												placeholder="再次输入密码"
												required
												autoComplete="new-password"
												onChange={(e) => setConfirmPassword(e.target.value)}
												onBlur={() =>
													setTouched((p) => ({ ...p, confirm: true }))
												}
											/>
											{confirmPasswordError && (
												<p className="text-destructive absolute top-full left-1 mt-1 text-xs">
													{confirmPasswordError}
												</p>
											)}
										</label>
										<label className="mb-4 block">
											<span className="mb-1 block text-sm">邮箱</span>
											<Input
												type="email"
												value={email}
												placeholder="请输入邮箱"
												autoComplete="email"
												onChange={(e) => setEmail(e.target.value)}
											/>
											<p className="text-faint-foreground mt-1 text-xs">
												选填。若服务器开启邮箱验证，则邮箱与验证码必填
											</p>
										</label>
										{email.trim() && (
											<label className="relative mb-4 block">
												<span className="mb-1 block text-sm">验证码</span>
												<div className="flex items-center gap-2">
													<Input
														type="text"
														value={emailCode}
														placeholder="请输入验证码"
														autoComplete="one-time-code"
														onChange={(e) => setEmailCode(e.target.value)}
													/>
													<Button
														type="button"
														variant="outline"
														className="shrink-0"
														disabled={
															codeSending || codeCountdown > 0 || !email.trim()
														}
														onClick={handleSendCode}
													>
														{codeSending
															? "发送中..."
															: codeCountdown > 0
																? `${codeCountdown}s 后重发`
																: "发送验证码"}
													</Button>
												</div>
											</label>
										)}

										{errorMsg && (
											<div
												role="alert"
												className="border-destructive/50 bg-destructive/10 text-destructive mb-2 flex items-center gap-2 rounded-sm px-3 py-2 text-sm"
											>
												<span>{errorMsg}</span>
											</div>
										)}

										<Button
											type="submit"
											size="lg"
											className="mt-4 w-full"
											disabled={submitting}
										>
											{submitting ? "注册中..." : "注册"}
										</Button>
									</form>

									<div className="mt-1 text-center">
										<Link
											to="/login"
											className="text-primary inline-flex min-h-11 items-center justify-center px-2 text-sm hover:underline"
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
