import { Switch, buttonVariants, toast } from "@gospeak/components";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Mail, RefreshCcw, Save } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import {
	type EmailConfigInput,
	getEmailConfig,
	updateEmailConfig,
} from "@/api/email";
import {
	ManageHeader,
	ManageLoading,
	ManageNotice,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import PasswordInput from "@/components/common/passwordInput";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";

export const Route = createFileRoute("/(app)/manage/email/")({
	beforeLoad: () => {
		if (!hasPermission("email_config:read")) {
			denyAccess();
		}
	},
	component: EmailPage,
	staticData: {
		title: "邮箱",
	},
});

const inputClass =
	"border-input bg-card text-foreground focus-visible:ring-ring placeholder:text-faint-foreground flex h-9 w-full rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50";

function Field({ label, children }: { label: string; children: ReactNode }) {
	return (
		<label className="block">
			<span className={labelSpan}>{label}</span>
			{children}
		</label>
	);
}

const labelSpan = "mb-1 block text-sm";
const btn = buttonVariants({ variant: "outline" });

function EmailPage() {
	const queryClient = useQueryClient();
	const {
		data: config,
		isLoading,
		error,
		refetch,
	} = useQuery({
		queryKey: ["email-config"],
		queryFn: getEmailConfig,
	});

	const [enabled, setEnabled] = useState(false);
	const [smtpHost, setSmtpHost] = useState("");
	const [smtpPort, setSmtpPort] = useState("587");
	const [smtpUsername, setSmtpUsername] = useState("");
	const [smtpPassword, setSmtpPassword] = useState("");
	const [smtpFrom, setSmtpFrom] = useState("");
	const [smtpFromName, setSmtpFromName] = useState("GoSpeak");
	const [emailCodeTTL, setEmailCodeTTL] = useState("10m");
	const [emailSendCooldown, setEmailSendCooldown] = useState("60s");
	const [emailCodeSecret, setEmailCodeSecret] = useState("");
	const [smtpPasswordSet, setSmtpPasswordSet] = useState(false);
	const [emailCodeSecretSet, setEmailCodeSecretSet] = useState(false);
	const [saving, setSaving] = useState(false);

	useEffect(() => {
		const data = config;
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
	}, [config]);

	const handleSave = async () => {
		if (saving) return;
		setSaving(true);
		try {
			const input: EmailConfigInput = {
				enabled,
				smtp_host: smtpHost,
				smtp_port: smtpPort,
				smtp_username: smtpUsername,
				smtp_from: smtpFrom,
				smtp_from_name: smtpFromName,
				email_code_ttl: emailCodeTTL,
				email_send_cooldown: emailSendCooldown,
			};
			if (smtpPassword) input.smtp_password = smtpPassword;
			if (emailCodeSecret) input.email_code_secret = emailCodeSecret;

			const saved = await updateEmailConfig(input);
			setSmtpPasswordSet(!!saved.smtp_password_set);
			setEmailCodeSecretSet(!!saved.email_code_secret_set);
			setSmtpPassword("");
			setEmailCodeSecret("");
			// invalidate 已经会触发重新拉取，再 refetch 一次等于打两遍接口
			await queryClient.invalidateQueries({ queryKey: ["email-config"] });
			toast.success("邮箱配置已保存");
		} catch (error) {
			toastActionError(error, "保存邮箱配置失败");
		} finally {
			setSaving(false);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Mail size={20} />}
				title="邮箱配置"
				description="SMTP 与邮箱验证码参数"
				actions={
					<button
						type="button"
						className="hover:bg-muted flex size-9 cursor-pointer items-center justify-center rounded-sm transition-colors disabled:pointer-events-none disabled:opacity-50"
						onClick={() => void refetch()}
						disabled={saving || isLoading}
						title="重新加载"
					>
						<RefreshCcw size={16} />
					</button>
				}
			/>

			{isLoading && <ManageLoading />}

			{!isLoading && error && (
				<div className="border-destructive/50 bg-destructive/10 text-destructive flex items-center gap-2 rounded-sm px-3 py-2 text-sm">
					{error instanceof Error ? error.message : "加载失败"}
				</div>
			)}

			{!isLoading && config && (
				<>
					<ManageNotice tone={config.available ? "info" : "warning"}>
						{config.available
							? "邮箱验证能力已就绪，注册和重置密码均使用验证码流程。"
							: "邮箱验证未就绪。配置完整 SMTP 并开启启用开关后，注册将要求邮箱验证码，重置密码将切换为邮箱验证码流程。"}
					</ManageNotice>

					<section className="border-border/80 bg-card flex items-center justify-between gap-4 rounded-xl border px-4 py-3 shadow-sm md:px-5">
						<div className="min-w-0">
							<div className="text-foreground text-sm font-semibold">
								启用邮箱验证
							</div>
							<p className="text-faint-foreground mt-0.5 text-xs">
								关闭时保持现有注册与登录行为，重置密码将被禁用
							</p>
						</div>
						<Switch
							checked={enabled}
							onCheckedChange={setEnabled}
							aria-label="启用邮箱验证"
						/>
					</section>

					<ManageSection title="SMTP 配置" description="发信服务器与身份">
						<div className="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
							<Field label="SMTP Host">
								<input
									type="text"
									className={inputClass}
									placeholder="smtp.qq.com"
									value={smtpHost}
									onChange={(e) => setSmtpHost(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
							<Field label="SMTP Port">
								<input
									type="text"
									className={inputClass}
									placeholder="587"
									value={smtpPort}
									onChange={(e) => setSmtpPort(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
							<Field label="用户名">
								<input
									type="text"
									className={inputClass}
									placeholder="example@qq.com"
									value={smtpUsername}
									onChange={(e) => setSmtpUsername(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
							<Field label="密码 / 授权码">
								<PasswordInput
									native
									autoComplete="off"
									className={inputClass}
									placeholder={
										smtpPasswordSet ? "已配置，留空保留" : "SMTP 密码"
									}
									value={smtpPassword}
									onChange={(e) => setSmtpPassword(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
							<Field label="发件地址">
								<input
									type="text"
									className={inputClass}
									placeholder="example@qq.com"
									value={smtpFrom}
									onChange={(e) => setSmtpFrom(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
							<Field label="发件人名称">
								<input
									type="text"
									className={inputClass}
									placeholder="GoSpeak"
									value={smtpFromName}
									onChange={(e) => setSmtpFromName(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
						</div>
					</ManageSection>

					<ManageSection
						title="验证码参数"
						description="有效期、冷却与签名密钥"
						actions={
							<button
								type="button"
								className={btn}
								disabled={saving}
								onClick={handleSave}
							>
								<Save size={16} />
								{saving ? "保存中..." : "保存配置"}
							</button>
						}
					>
						<div className="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
							<Field label="验证码有效期">
								<input
									type="text"
									className={inputClass}
									placeholder="10m"
									value={emailCodeTTL}
									onChange={(e) => setEmailCodeTTL(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
							<Field label="发送冷却时间">
								<input
									type="text"
									className={inputClass}
									placeholder="60s"
									value={emailSendCooldown}
									onChange={(e) => setEmailSendCooldown(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
							<Field label="验证码签名密钥">
								<PasswordInput
									native
									autoComplete="off"
									className={inputClass}
									placeholder={
										emailCodeSecretSet ? "已配置，留空保留" : "验证码签名密钥"
									}
									value={emailCodeSecret}
									onChange={(e) => setEmailCodeSecret(e.currentTarget.value)}
									disabled={saving}
								/>
							</Field>
						</div>
					</ManageSection>
				</>
			)}
		</ManagePage>
	);
}
