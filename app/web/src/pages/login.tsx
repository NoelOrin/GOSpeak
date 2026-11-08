import { Button, Input, toast } from "@gospeak/components";
import {
	Link,
	createFileRoute,
	redirect,
	useNavigate,
} from "@tanstack/react-router";
import { CircleX, Loader2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
	firstChangePassword as firstChangePasswordApi,
	login as loginApi,
} from "@/api/auth";
import { getEnabledProviders, getOAuthLoginURL } from "@/api/oauth";
import PasswordInput from "@/components/common/passwordInput";
import ServerUrlField from "@/components/common/serverUrlField";
import ProviderIcon from "@/components/oauth/ProviderIcon";
import { useUserStore } from "@/stores/userStore";
import { inDesktop } from "@gospeak/desktop";
import { depth, useAuthParallax } from "@/hooks/useAuthParallax";
import ForcePasswordChangeModal from "@/pages/login/components/ForcePasswordChangeModal";
import ForgotPasswordModal from "@/pages/login/components/ForgotPasswordModal";
import { showLoginSuccessToast } from "@/pages/login/components/loginSuccessToast";

export const Route = createFileRoute("/login")({
	beforeLoad: async () => {
		// 已有会话或 access 过期但仍可无感刷新时，直接进首页
		const ok = await useUserStore.getState().ensureSession();
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
	const shellRef = useRef<HTMLDivElement>(null);
	useAuthParallax(shellRef);

	const [banned, setBanned] = useState(false);
	const [showForgotModal, setShowForgotModal] = useState(false);
	const [showChangeModal, setShowChangeModal] = useState(false);
	const [oauthLoading, setOauthLoading] = useState(false);

	const { data: oauthProviders } = useQuery({
		queryKey: ["oauth-providers"],
		queryFn: getEnabledProviders,
	});

	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [touched, setTouched] = useState({ username: false, password: false });

	const usernameError =
		touched.username && !username.trim() ? "用户名 是必填项" : "";
	const passwordError = touched.password && !password ? "密码 是必填项" : "";

	function completeOAuthLogin(expiresIn?: number) {
		void (async () => {
			setOauthLoading(true);
			try {
				// token 已由服务端写入 HttpOnly Cookie，这里先记录过期时间再确认会话并拉取 profile。
				useUserStore.getState().recordSessionExpiry(expiresIn);
				const ok = await useUserStore.getState().ensureSession();
				if (!ok) {
					toast.error("OAuth 登录失败：未能建立会话");
					return;
				}
				showLoginSuccessToast();
				navigate({ to: resolveLoginRedirect() });
			} catch (e) {
				await useUserStore.getState().clearAuth();
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

	useEffect(() => {
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

		const params = new URLSearchParams(window.location.search);
		if (params.get("banned") === "1") {
			setBanned(true);
			window.history.replaceState({}, "", "/login");
			return;
		}

		const oauthError = params.get("oauth_error");
		if (oauthError) {
			toast.error(oauthError);
			window.history.replaceState({}, "", "/login");
			return;
		}

		window.addEventListener("message", handleOAuthMessage);
		return () => window.removeEventListener("message", handleOAuthMessage);
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	async function handleLoginSubmit(e?: React.FormEvent) {
		e?.preventDefault();
		e?.stopPropagation();
		setTouched({ username: true, password: true });
		if (!username.trim() || !password) return;
		if (submitting) return;
		setSubmitting(true);
		try {
			const data = await loginApi({
				username: username.trim(),
				password,
			});
			if (data.need_change_password) {
				await useUserStore.getState().login(data.user, data.expires_in);
				setShowChangeModal(true);
				return;
			}
			await useUserStore.getState().login(data.user, data.expires_in);
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

	const providers = oauthProviders ?? [];

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
							<p className="text-faint-foreground mt-2 text-sm">登录你的账号</p>
						</div>
						<div className="card-tilt">
							<div className="border-foreground/10 bg-card rounded-lg border shadow-xl shadow-black/20">
								<div className="p-6 sm:p-8">
									<div className="mb-6 hidden items-center justify-between md:flex">
										<h2 className="text-lg font-semibold">登录账号</h2>
										<span className="text-faint-foreground font-mono text-2xs uppercase">
											Login
										</span>
									</div>

									<form onSubmit={handleLoginSubmit} noValidate>
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
												<p className="text-destructive mt-1 block text-xs">
													{usernameError}
												</p>
											)}
										</label>
										<label className="relative mb-4 block">
											<span className="mb-1 block text-sm">密码</span>
											<PasswordInput
												value={password}
												placeholder="请输入密码"
												required
												autoComplete="current-password"
												onChange={(e) => setPassword(e.target.value)}
												onBlur={() =>
													setTouched((p) => ({ ...p, password: true }))
												}
											/>
											{passwordError && (
												<p className="text-destructive mt-1 block text-xs">
													{passwordError}
												</p>
											)}
										</label>
										<Button
											type="submit"
											size="lg"
											className="mt-4 w-full"
											disabled={submitting}
										>
											{submitting ? "登录中..." : "登录"}
										</Button>
									</form>

									<div className="mt-1 flex items-center justify-center">
										<button
											type="button"
											className="text-primary inline-flex min-h-11 items-center justify-center px-2 text-sm hover:underline"
											onClick={() => setShowForgotModal(true)}
										>
											忘记密码?
										</button>
										<span className="text-faint-foreground">·</span>
										<Link
											to="/register"
											className="text-primary inline-flex min-h-11 items-center justify-center px-2 text-sm hover:underline"
										>
											注册账号
										</Link>
									</div>

									{/* 已启用的第三方登录：按提供商显示品牌 icon 按钮 */}
									{providers.length > 0 && (
										<>
											<div className="text-faint-foreground my-3 flex items-center gap-2 text-xs">
												<span className="bg-border h-px flex-1" />
												或使用第三方登录
												<span className="bg-border h-px flex-1" />
											</div>
											<div className="flex flex-wrap items-center justify-center gap-3">
												{providers.map((p) => {
													const label = p.display_name || p.name;
													return (
														<Button
															key={p.name}
															variant="outline"
															className="size-12 p-0"
															title={`使用 ${label} 登录`}
															aria-label={`使用 ${label} 登录`}
															disabled={oauthLoading}
															onClick={() => {
																setOauthLoading(true);
																const win = window.open(
																	getOAuthLoginURL(p.name),
																	"gospeak-oauth",
																	"popup,width=560,height=640",
																);
																if (!win) {
																	setOauthLoading(false);
																	toast.error("请允许弹窗后重试 OAuth 登录");
																}
															}}
														>
															<ProviderIcon
																name={p.name}
																iconUrl={p.icon_url}
																size={22}
															/>
														</Button>
													);
												})}
											</div>
										</>
									)}

									{oauthLoading && (
										<div className="text-muted-foreground mt-3 flex items-center justify-center gap-2 text-sm">
											<Loader2 className="size-4 animate-spin" />
											正在完成第三方登录…
										</div>
									)}

									{banned && (
										<div className="border-destructive/50 bg-destructive/10 text-destructive mt-2 flex items-center gap-2 rounded-sm px-3 py-2">
											<CircleX className="size-5 shrink-0" />
											<span className="text-sm">
												您的账号已被封禁，无法登录。如有疑问请联系管理员。
											</span>
										</div>
									)}

									{/* 桌面端：登录前即可指定要连接的服务器（持久化，设置里也可改） */}
									{inDesktop() && (
										<div className="border-foreground/10 mt-4 border-t pt-4">
											<ServerUrlField />
										</div>
									)}
								</div>
							</div>
						</div>
					</div>
				</main>
			</div>

			{/* 忘记密码 Modal */}
			<ForgotPasswordModal
				open={showForgotModal}
				onClose={() => setShowForgotModal(false)}
			/>

			{/* Admin 首次登录强制改密 Modal */}
			<ForcePasswordChangeModal
				open={showChangeModal}
				onClose={() => setShowChangeModal(false)}
				onSubmit={async ({ newPassword, name }) => {
					const result = await firstChangePasswordApi(newPassword ?? "", name);
					await useUserStore.getState().login(result.user, result.expires_in);
					setShowChangeModal(false);
					showLoginSuccessToast();
					navigate({ to: resolveLoginRedirect() });
				}}
			/>
		</div>
	);
}
