import { createFileRoute, redirect } from "@tanstack/solid-router";
import Mail from "lucide-solid/icons/mail";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Save from "lucide-solid/icons/save";
import { createEffect, createResource, createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	type EmailConfigInput,
	getEmailConfig,
	updateEmailConfig,
} from "@/api/email";
import { hasPermission } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/manage/email/")({
	beforeLoad: () => {
		if (!hasPermission("email_config:read")) {
			throw redirect({ to: "/" });
		}
	},
	component: EmailPage,
	staticData: {
		title: "邮箱",
		icon: "icon-manage",
	},
});

function EmailPage() {
	const [config, { refetch }] = createResource(getEmailConfig);
	const [enabled, setEnabled] = createSignal(false);
	const [smtpHost, setSmtpHost] = createSignal("");
	const [smtpPort, setSmtpPort] = createSignal("587");
	const [smtpUsername, setSmtpUsername] = createSignal("");
	const [smtpPassword, setSmtpPassword] = createSignal("");
	const [smtpFrom, setSmtpFrom] = createSignal("");
	const [smtpFromName, setSmtpFromName] = createSignal("GoSpeak");
	const [emailCodeTTL, setEmailCodeTTL] = createSignal("10m");
	const [emailSendCooldown, setEmailSendCooldown] = createSignal("60s");
	const [emailCodeSecret, setEmailCodeSecret] = createSignal("");
	const [smtpPasswordSet, setSmtpPasswordSet] = createSignal(false);
	const [emailCodeSecretSet, setEmailCodeSecretSet] = createSignal(false);
	const [saving, setSaving] = createSignal(false);

	createEffect(() => {
		const data = config();
		if (!data) return;
		setEnabled(data.enabled);
		setSmtpHost(data.smtp_host || "");
		setSmtpPort(data.smtp_port || "587");
		setSmtpUsername(data.smtp_username || "");
		setSmtpFrom(data.smtp_from || "");
		setSmtpFromName(data.smtp_from_name || "GoSpeak");
		setEmailCodeTTL(data.email_code_ttl || "10m");
		setEmailSendCooldown(data.email_send_cooldown || "60s");
		setSmtpPasswordSet(!!data.smtp_password_set);
		setEmailCodeSecretSet(!!data.email_code_secret_set);
	});

	const handleSave = async () => {
		setSaving(true);
		try {
			const input: EmailConfigInput = {
				enabled: enabled(),
				smtp_host: smtpHost(),
				smtp_port: smtpPort(),
				smtp_username: smtpUsername(),
				smtp_from: smtpFrom(),
				smtp_from_name: smtpFromName(),
				email_code_ttl: emailCodeTTL(),
				email_send_cooldown: emailSendCooldown(),
			};
			if (smtpPassword()) input.smtp_password = smtpPassword();
			if (emailCodeSecret()) input.email_code_secret = emailCodeSecret();

			const saved = await updateEmailConfig(input);
			setSmtpPasswordSet(!!saved.smtp_password_set);
			setEmailCodeSecretSet(!!saved.email_code_secret_set);
			showToast("邮箱配置已保存", { type: "success" });
			setSmtpPassword("");
			setEmailCodeSecret("");
			await refetch();
		} catch (error) {
			showToast(error instanceof Error ? error.message : "保存失败", {
				type: "error",
			});
		} finally {
			setSaving(false);
		}
	};

	return (
		<Show
			when={!config.loading}
			fallback={
				<div class="flex h-full min-h-52 items-center justify-center">
					<span class="loading loading-spinner loading-lg" />
				</div>
			}
		>
			<div class="p-4 flex flex-col gap-4">
				<div class="flex items-center justify-between gap-3">
					<div class="flex items-center gap-2">
						<Mail size={20} />
						<h3 class="font-bold text-lg">邮箱配置</h3>
					</div>
					<button
						class="btn btn-ghost btn-sm"
						onClick={() => void refetch()}
						disabled={saving()}
						title="重新加载"
					>
						<RefreshCcw size={16} />
					</button>
				</div>

				<Show when={config.error}>
					<div class="alert alert-error text-sm">
						{config.error instanceof Error ? config.error.message : "加载失败"}
					</div>
				</Show>

				<Show when={config()}>
					{(data) => (
						<div
							class="rounded-box border border-base-300 bg-base-200/40 px-4 py-3 text-sm text-base-content/70"
							classList={{
								"border-success/30 bg-success/8": data().available,
								"border-warning/20 bg-warning/8": !data().available,
							}}
						>
							{data().available
								? "邮箱验证能力已就绪，注册和重置密码均使用验证码流程。"
								: "邮箱验证未就绪。配置完整 SMTP 并开启启用开关后，注册将要求邮箱验证码，重置密码将切换为邮箱验证码流程。"}
						</div>
					)}
				</Show>

				<div class="divider my-0 text-xs text-base-content/40">功能开关</div>

				<div class="flex items-center justify-between py-2">
					<div>
						<div class="text-sm font-medium">启用邮箱验证</div>
						<div class="text-xs text-base-content/50">
							关闭时保持现有注册与登录行为，重置密码将被禁用
						</div>
					</div>
					<input
						type="checkbox"
						class="toggle toggle-sm"
						checked={enabled()}
						onChange={(e) => setEnabled(e.currentTarget.checked)}
					/>
				</div>

				<div class="divider my-0 text-xs text-base-content/40">SMTP 配置</div>

				<div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
					<Field label="SMTP Host">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="smtp.qq.com"
							value={smtpHost()}
							onInput={(e) => setSmtpHost(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
					<Field label="SMTP Port">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="587"
							value={smtpPort()}
							onInput={(e) => setSmtpPort(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
					<Field label="用户名">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="example@qq.com"
							value={smtpUsername()}
							onInput={(e) => setSmtpUsername(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
					<Field label="密码 / 授权码">
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							placeholder={smtpPasswordSet() ? "已配置，留空保留" : "SMTP 密码"}
							value={smtpPassword()}
							onInput={(e) => setSmtpPassword(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
					<Field label="发件地址">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="example@qq.com"
							value={smtpFrom()}
							onInput={(e) => setSmtpFrom(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
					<Field label="发件人名称">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="GoSpeak"
							value={smtpFromName()}
							onInput={(e) => setSmtpFromName(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
				</div>

				<div class="divider my-0 text-xs text-base-content/40">验证码参数</div>

				<div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
					<Field label="验证码有效期">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="10m"
							value={emailCodeTTL()}
							onInput={(e) => setEmailCodeTTL(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
					<Field label="发送冷却时间">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="60s"
							value={emailSendCooldown()}
							onInput={(e) => setEmailSendCooldown(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
					<Field label="验证码签名密钥">
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							placeholder={
								emailCodeSecretSet() ? "已配置，留空保留" : "验证码签名密钥"
							}
							value={emailCodeSecret()}
							onInput={(e) => setEmailCodeSecret(e.currentTarget.value)}
							disabled={saving()}
						/>
					</Field>
				</div>

				<div class="flex justify-end pt-2">
					<button
						type="button"
						class="btn btn-primary btn-sm"
						classList={{ "btn-disabled": saving() }}
						onClick={handleSave}
					>
						<Show when={saving()} fallback={<Save size={16} />}>
							<span class="loading loading-spinner loading-xs" />
						</Show>
						{saving() ? "保存中..." : "保存配置"}
					</button>
				</div>
			</div>
		</Show>
	);
}

interface FieldProps {
	label: string;
	children: any;
}

const Field = (props: FieldProps) => (
	<fieldset class="fieldset">
		<legend class="fieldset-legend text-[14px]">{props.label}</legend>
		{props.children}
	</fieldset>
);
