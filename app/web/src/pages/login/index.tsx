import { createForm } from "@tanstack/solid-form";
import { createFileRoute, redirect, useNavigate } from "@tanstack/solid-router";
import { createSignal, onMount, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	firstChangePassword as firstChangePasswordApi,
	login as loginApi,
	resetPassword as resetPasswordApi,
} from "@/api/auth";
import { sendEmailCode } from "@/api/email";
import { Form } from "@/components/form";
import PasswordChangeForm from "@/components/form/PasswordChangeForm";
import userStore from "@/stores/userStore";

export const Route = createFileRoute("/login/")({
	beforeLoad: () => {
		if (userStore.isLoggedIn()) {
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

	onMount(() => {
		const params = new URLSearchParams(window.location.search);
		if (params.get("banned") === "1") {
			setBanned(true);
			window.history.replaceState({}, "", "/login");
		}
	});
	const [showChangeModal, setShowChangeModal] = createSignal(false);
	const [loginData, setLoginData] = createSignal<Awaited<
		ReturnType<typeof loginApi>
	> | null>(null);

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
					setLoginData(data);
					openChangeModal();
					return;
				}
				await userStore.login(data.user, data.access_token, data.refresh_token);
				navigate({ to: "/" });
			} catch (e: any) {
				if (e?.response?.data?.code === 1015) {
					setBanned(true);
				} else {
					showToast(
						e?.response?.data?.msg || e?.message || "登录失败，请重试",
						{ type: "error" },
					);
				}
			}
		},
	}));

	return (
		<div class="flex items-center justify-center w-screen h-screen bg-base-200">
			<div class="card w-96 bg-base-100 shadow-xl">
				<div class="card-body">
					<div class="text-center mb-2">
						<h1 class="text-3xl font-bold tracking-tight">GOSpeak</h1>
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
						<button class="link link-primary text-sm" onClick={openForgotModal}>
							忘记密码?
						</button>
					</div>

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
			<Show when={showForgotModal()}>
				<dialog ref={forgotDialogRef} class="modal" onClose={closeForgotModal}>
					<div class="modal-box">
						<h3 class="font-bold text-lg mb-4">重置密码</h3>
						<Show
							when={forgotStep() === "email"}
							fallback={
								<form
									onSubmit={async (e) => {
										e.preventDefault();
										const formEl = e.target as HTMLFormElement;
										const pwd = (formEl.querySelector('input[name="newPassword"]') as HTMLInputElement).value;
										try {
											await resetPasswordApi(forgotEmail(), forgotCode(), pwd);
											closeForgotModal();
											showToast("密码已重置，请登录", { type: "success" });
										} catch (err: any) {
											showToast(err?.response?.data?.msg || err?.message || "重置失败", { type: "error" });
										}
									}}
								>
									<fieldset class="fieldset mb-3">
										<legend class="fieldset-legend text-[14px]">邮箱</legend>
										<input type="email" value={forgotEmail()} disabled class="input w-full" />
									</fieldset>
									<fieldset class="fieldset mb-3">
										<legend class="fieldset-legend text-[14px]">验证码</legend>
										<input type="text" placeholder="请输入邮箱验证码" class="input w-full" onInput={(e) => setForgotCode(e.currentTarget.value)} />
									</fieldset>
									<fieldset class="fieldset mb-3">
										<legend class="fieldset-legend text-[14px]">新密码</legend>
										<input type="password" name="newPassword" placeholder="请输入新密码" class="input w-full" />
									</fieldset>
									<button type="submit" class="btn btn-primary w-full">重置密码</button>
								</form>
							}
						>
							<form
								onSubmit={async (e) => {
									e.preventDefault();
									setCodeSending(true);
									try {
										await sendEmailCode({ email: forgotEmail(), scene: "reset_password" });
										setForgotStep("code");
										showToast("验证码已发送", { type: "success" });
									} catch (err: any) {
										showToast(err?.response?.data?.msg || err?.message || "发送失败", { type: "error" });
									} finally {
										setCodeSending(false);
									}
								}}
							>
								<fieldset class="fieldset mb-3">
									<legend class="fieldset-legend text-[14px]">邮箱</legend>
									<input type="email" required placeholder="请输入注册邮箱" class="input w-full" value={forgotEmail()} onInput={(e) => setForgotEmail(e.currentTarget.value)} />
								</fieldset>
								<button type="submit" class="btn btn-primary w-full" disabled={codeSending()}>
									{codeSending() ? "发送中..." : "发送验证码"}
								</button>
							</form>
						</Show>
						<div class="modal-action">
							<form method="dialog">
								<button class="btn" onClick={closeForgotModal}>取消</button>
							</form>
						</div>
					</div>
					<form method="dialog" class="modal-backdrop">
						<button onClick={closeForgotModal}>close</button>
					</form>
				</dialog>
			</Show>

			{/* Admin 首次登录强制改密 Modal */}
			<Show when={showChangeModal()}>
				<dialog ref={changeDialogRef} class="modal" onClose={closeChangeModal}>
					<div class="modal-box">
						<h3 class="font-bold text-lg mb-2">修改密码</h3>
						<p class="text-sm text-base-content/60 mb-4">
							检测到您使用的是默认密码，请修改密码后继续。
						</p>
						<PasswordChangeForm
							showOldPassword={false}
							showName={true}
							submitText="修改密码"
							onSubmit={async ({ newPassword, name }) => {
								const result = await firstChangePasswordApi(
									newPassword!,
									name,
								);
								await userStore.login(result.user, result.access_token, result.refresh_token);
								closeChangeModal();
								navigate({ to: "/" });
							}}
						/>
					</div>
					<form method="dialog" class="modal-backdrop">
						<button>close</button>
					</form>
				</dialog>
			</Show>
		</div>
	);
}
