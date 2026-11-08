import apiClient from "./apiClient";

export interface EmailConfigView {
	enabled: boolean;
	smtp_host: string;
	smtp_port: string;
	smtp_username: string;
	smtp_password: string;
	smtp_password_set?: boolean;
	smtp_from: string;
	smtp_from_name: string;
	email_code_ttl: string;
	email_send_cooldown: string;
	email_code_secret: string;
	email_code_secret_set?: boolean;
	available: boolean;
}

export interface EmailConfigInput {
	enabled: boolean;
	smtp_host?: string;
	smtp_port?: string;
	smtp_username?: string;
	smtp_password?: string;
	smtp_from?: string;
	smtp_from_name?: string;
	email_code_ttl?: string;
	email_send_cooldown?: string;
	email_code_secret?: string;
}

export interface SendEmailCodeInput {
	email: string;
	scene: "register" | "reset_password" | "bind_email" | "change_email";
}

export interface SendEmailCodeResult {
	expires_in: number;
}

export interface VerifyEmailCodeInput extends SendEmailCodeInput {
	code: string;
}

export async function getEmailConfig(): Promise<EmailConfigView> {
	const data = await apiClient.post<EmailConfigView>({
		url: "/api/v1/email/config",
	});
	if (!data) throw new Error("email config is missing");
	return data;
}

export async function updateEmailConfig(
	config: EmailConfigInput,
): Promise<EmailConfigView> {
	const data = await apiClient.post<EmailConfigView>({
		url: "/api/v1/email/update-config",
		data: config,
	});
	if (!data) throw new Error("email config is missing");
	return data;
}

export async function sendEmailCode(
	input: SendEmailCodeInput,
	opts?: { skipErrorToast?: boolean },
): Promise<SendEmailCodeResult> {
	const data = await apiClient.post<SendEmailCodeResult>({
		url: "/api/v1/email/send_code",
		data: input,
		// 调用方按业务码定制中文文案时不重复提示
		skipErrorToast: opts?.skipErrorToast,
	});
	if (!data) throw new Error("send email code response is missing");
	return data;
}
