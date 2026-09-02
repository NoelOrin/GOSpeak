import { Show } from "solid-js";
import { showToast } from "solid-notifications";
import { resetPassword } from "@/api/auth";
import { sendEmailCode } from "@/api/email";

export interface ForgotPasswordModalProps {
	open: boolean;
	email: string;
	code: string;
	step: "email" | "code";
	codeSending: boolean;
	dialogRef: (el: HTMLDialogElement) => void;
	onClose: () => void;
	setEmail: (v: string) => void;
	setCode: (v: string) => void;
	setStep: (v: "email" | "code") => void;
	setCodeSending: (v: boolean) => void;
}

export default function ForgotPasswordModal(props: ForgotPasswordModalProps) {
	return (
		<Show when={props.open}>
			<dialog
				ref={(el) => {
					props.dialogRef(el);
				}}
				class="modal"
				onClose={props.onClose}
			>
				<div class="modal-box">
					<h3 class="font-bold text-lg mb-4">重置密码</h3>
					<Show
						when={props.step === "email"}
						fallback={
							<form
								onSubmit={async (e) => {
									e.preventDefault();
									const formEl = e.target as HTMLFormElement;
									const pwd = (
										formEl.querySelector(
											'input[name="newPassword"]',
										) as HTMLInputElement
									).value;
									try {
										await resetPassword(props.email, props.code, pwd);
										props.onClose();
										showToast("密码已重置，请登录", { type: "success" });
									} catch {}
								}}
							>
								<fieldset class="fieldset mb-3">
									<legend class="fieldset-legend text-[14px]">邮箱</legend>
									<input
										type="email"
										value={props.email}
										disabled
										class="input w-full"
									/>
								</fieldset>
								<fieldset class="fieldset mb-3">
									<legend class="fieldset-legend text-[14px]">验证码</legend>
									<input
										type="text"
										placeholder="请输入邮箱验证码"
										class="input w-full"
										onInput={(e) => props.setCode(e.currentTarget.value)}
									/>
								</fieldset>
								<fieldset class="fieldset mb-3">
									<legend class="fieldset-legend text-[14px]">新密码</legend>
									<input
										type="password"
										name="newPassword"
										placeholder="请输入新密码"
										class="input w-full"
									/>
								</fieldset>
								<button type="submit" class="btn btn-primary w-full">
									重置密码
								</button>
							</form>
						}
					>
						<form
							onSubmit={async (e) => {
								e.preventDefault();
								props.setCodeSending(true);
								try {
									await sendEmailCode({
										email: props.email,
										scene: "reset_password",
									});
									props.setStep("code");
									showToast("验证码已发送", { type: "success" });
								} catch {
								} finally {
									props.setCodeSending(false);
								}
							}}
						>
							<fieldset class="fieldset mb-3">
								<legend class="fieldset-legend text-[14px]">邮箱</legend>
								<input
									type="email"
									required
									placeholder="请输入注册邮箱"
									class="input w-full"
									value={props.email}
									onInput={(e) => props.setEmail(e.currentTarget.value)}
								/>
							</fieldset>
							<button
								type="submit"
								class="btn btn-primary w-full"
								disabled={props.codeSending}
							>
								{props.codeSending ? "发送中..." : "发送验证码"}
							</button>
						</form>
					</Show>
					<div class="modal-action">
						<form method="dialog">
							<button class="btn" onClick={props.onClose}>
								取消
							</button>
						</form>
					</div>
				</div>
				<form method="dialog" class="modal-backdrop">
					<button onClick={props.onClose}>close</button>
				</form>
			</dialog>
		</Show>
	);
}
