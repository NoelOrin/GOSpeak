import type { AxiosResponse } from "axios";
import apiClient, { type Result } from "./apiClient";

export interface EmailConfigView {
	enabled: boolean;
	smtp_host: string;
	smtp_port: string;
	smtp_username: string;
	smtp_password: string;
	smtp_from: string;
	smtp_from_name: string;
	email_code_ttl: string;
	email_send_cooldown: string;
	email_code_secret: string;
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
	const res = (await apiClient.post({
		url: "/api/v1/email/config",
	})) as AxiosResponse<Result<EmailConfigView>>;
	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("email config is missing");
	return result.data;
}

export async function updateEmailConfig(config: EmailConfigInput): Promise<EmailConfigView> {
	const res = (await apiClient.post({
		url: "/api/v1/email/update-config",
		data: config,
	})) as AxiosResponse<Result<EmailConfigView>>;
	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("email config is missing");
	return result.data;
}

export async function sendEmailCode(input: SendEmailCodeInput): Promise<SendEmailCodeResult> {
	const res = (await apiClient.post({
		url: "/api/v1/email/send_code",
		data: input,
	})) as AxiosResponse<Result<SendEmailCodeResult>>;
	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("send email code response is missing");
	return result.data;
}

export async function verifyEmailCode(input: VerifyEmailCodeInput): Promise<{ verified: boolean }> {
	const res = (await apiClient.post({
		url: "/api/v1/email/verify_code",
		data: input,
	})) as AxiosResponse<Result<{ verified: boolean }>>;
	const result = res.data;
	if (result.code !== 0) throw new Error(result.msg);
	if (!result.data) throw new Error("verify email code response is missing");
	return result.data;
}
