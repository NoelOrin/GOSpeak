import { createForm } from "@tanstack/solid-form";
import { createFileRoute, redirect, useNavigate } from "@tanstack/solid-router";
import { createResource, createSignal, For, onMount, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	firstChangePassword as firstChangePasswordApi,
	getProfile,
	login as loginApi,
} from "@/api/auth";
import { getEnabledProviders, getOAuthLoginURL } from "@/api/oauth";
import { Form } from "@/components/form";
import ProviderIcon from "@/components/oauth/ProviderIcon";
import userStore from "@/stores/userStore";
import ForcePasswordChangeModal from "./components/ForcePasswordChangeModal";
import ForgotPasswordModal from "./components/ForgotPasswordModal";

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

		// OAuth 回调：后端把 token 带回登录页，完成会话落地后进首页。
		if (params.get("oauth") === "1") {
			const accessToken = params.get("access_token") || "";
			const refreshToken = params.get("refresh_token") || "";
			window.history.replaceState({}, "", "/login");
			if (!accessToken || !refreshToken) {
				showToast("OAuth 登录失败：缺少 token", { type: "error" });
				return;
			}
			void (async () => {
				setOauthLoading(true);
				try {
					// 先写入 access token，供 getProfile 鉴权
					await userStore.login(
						{
							id: 0,
							uuid: "",
							name: "",
							display_name: "",
							avatar: "",
							role: "user",
						},
						accessToken,
						refreshToken,
					);
					const profile = await getProfile();
					await userStore.login(profile, accessToken, refreshToken);
					navigate({ to: "/" });
				} catch (e: any) {
					await userStore.clearAuth();
					if (e?.response?.data?.code === 1015) {
						setBanned(true);
					}
				} finally {
					setOauthLoading(false);
				}
			})();
		}
	});
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

	const form = createForm(() => ({
		defaultValues: { username: "", password: "" },
		onSubmit: async ({ value }) => {
			try {
				const data = await loginApi(value);
				if (data.need_change_password) {
					await userStore.login(
						data.user,
						data.access_token,
						data.refresh_token,
					);
					openChangeModal();
					return;
				}
				await userStore.login(data.user, data.access_token, data.refresh_token);
				navigate({ to: "/" });
			} catch (e: any) {
				if (e?.response?.data?.code === 1015) {
					setBanned(true);
				}
			}
		},
	}));

	return (
		<div class="flex items-center justify-center w-screen min-h-screen h-screen bg-base-200 p-4 overflow-y-auto">
			<div class="card w-full max-w-sm sm:max-w-md bg-base-100 shadow-xl">
				<div class="card-body p-5 sm:p-8">
					<div class="text-center mb-2">
						<h1 class="text-2xl sm:text-3xl font-bold tracking-tight">
							GOSpeak
						</h1>
						<p class="text-base-content/50 text-sm mt-1">登录你的账号</p>
					</div>

					<Form
						form={form}
						fields={[
							{
								name: "username",
								label: "用户名",
								type: "text",
								placeholder: "请输入用户名",
								required: true,
							},
							{
								name: "password",
								label: "密码",
								type: "password",
								placeholder: "请输入密码",
								required: true,
							},
						]}
						submitButtonText="登录"
					/>

					<div class="text-center mt-1">
						<button
							type="button"
							class="link link-primary text-sm"
							onClick={openForgotModal}
						>
							忘记密码?
						</button>
					</div>

					{/* 已启用的第三方登录：按提供商显示品牌 icon 按钮 */}
					<Show
						when={!oauthProviders.error && (oauthProviders()?.length || 0) > 0}
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
												window.location.href = getOAuthLoginURL(p.name);
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
							<svg
								xmlns="http://www.w3.org/2000/svg"
								class="stroke-current shrink-0 h-5 w-5"
								fill="none"
								viewBox="0 0 24 24"
							>
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="2"
									d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
								/>
							</svg>
							<span class="text-sm">
								您的账号已被封禁，无法登录。如有疑问请联系管理员。
							</span>
						</div>
					</Show>
				</div>
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
					await userStore.login(
						result.user,
						result.access_token,
						result.refresh_token,
					);
					closeChangeModal();
					navigate({ to: "/" });
				}}
			/>
		</div>
	);
}
