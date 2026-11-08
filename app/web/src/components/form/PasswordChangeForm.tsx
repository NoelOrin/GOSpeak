import { Button, Input } from "@gospeak/components";
import { useState } from "react";
import PasswordInput from "@/components/common/passwordInput";
import {
	fieldError,
	newPasswordSchema,
	requiredString,
} from "@/utils/formSchemas";

interface PasswordChangeFormProps {
	onSubmit: (values: {
		username?: string;
		oldPassword?: string;
		newPassword: string;
		name?: string;
	}) => Promise<void>;
	showUsername?: boolean;
	showOldPassword?: boolean;
	showName?: boolean;
	submitText?: string;
}

interface FieldErrors {
	username?: string;
	oldPassword?: string;
	newPassword?: string;
	confirmPassword?: string;
}

export default function PasswordChangeForm({
	onSubmit,
	showUsername,
	showOldPassword,
	showName,
	submitText,
}: PasswordChangeFormProps) {
	const [error, setError] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [values, setValues] = useState({
		username: "",
		oldPassword: "",
		newPassword: "",
		confirmPassword: "",
		name: "",
	});
	const [touched, setTouched] = useState({
		username: false,
		oldPassword: false,
		newPassword: false,
		confirmPassword: false,
	});

	const set = (key: keyof typeof values, v: string) =>
		setValues((prev) => ({ ...prev, [key]: v }));
	const touch = (key: keyof typeof touched) =>
		setTouched((prev) => ({ ...prev, [key]: true }));

	const validate = (
		key: keyof FieldErrors,
		message: string,
	): string | undefined => {
		if (!touched[key]) return undefined;
		return fieldError(requiredString(message), values[key]);
	};

	const errors: FieldErrors = {
		username: validate("username", "用户名 是必填项"),
		oldPassword: validate("oldPassword", "旧密码 是必填项"),
		newPassword: touched.newPassword
			? fieldError(newPasswordSchema, values.newPassword)
			: undefined,
		confirmPassword: validate("confirmPassword", "确认密码 是必填项"),
	};

	return (
		<form
			noValidate
			onSubmit={async (e) => {
				e.preventDefault();
				e.stopPropagation();
				setTouched({
					username: true,
					oldPassword: true,
					newPassword: true,
					confirmPassword: true,
				});
				if (
					(values.username && !values.username.trim()) ||
					(values.oldPassword && !values.oldPassword) ||
					!values.newPassword ||
					!values.confirmPassword
				) {
					return;
				}
				setError("");
				if (values.newPassword !== values.confirmPassword) {
					setError("两次输入的密码不一致");
					return;
				}
				setSubmitting(true);
				try {
					await onSubmit({
						username: showUsername ? values.username : undefined,
						oldPassword: showOldPassword ? values.oldPassword : undefined,
						newPassword: values.newPassword,
						name: showName ? values.name || undefined : undefined,
					});
				} catch (e: any) {
					setError(e?.response?.data?.msg || e?.message || "操作失败，请重试");
				} finally {
					setSubmitting(false);
				}
			}}
		>
			{showUsername && (
				<label className="mb-3 block">
					<span className="mb-1 block text-sm">用户名</span>
					<Input
						value={values.username}
						placeholder="请输入用户名"
						onChange={(e) => set("username", e.target.value)}
						onBlur={() => touch("username")}
					/>
					{errors.username && (
						<span className="text-destructive mt-1 block text-xs">
							{errors.username}
						</span>
					)}
				</label>
			)}

			{showName && (
				<label className="mb-3 block">
					<span className="mb-1 block text-sm">用户名（可选）</span>
					<Input
						value={values.name}
						placeholder="留空则保持不变"
						onChange={(e) => set("name", e.target.value)}
					/>
				</label>
			)}

			{showOldPassword && (
				<label className="mb-3 block">
					<span className="mb-1 block text-sm">旧密码</span>
					<PasswordInput
						autoComplete="current-password"
						value={values.oldPassword}
						placeholder="请输入旧密码"
						onChange={(e) => set("oldPassword", e.target.value)}
						onBlur={() => touch("oldPassword")}
					/>
					{errors.oldPassword && (
						<span className="text-destructive mt-1 block text-xs">
							{errors.oldPassword}
						</span>
					)}
				</label>
			)}

			<label className="mb-3 block">
				<span className="mb-1 block text-sm">新密码</span>
				<PasswordInput
					value={values.newPassword}
					placeholder="请输入新密码"
					onChange={(e) => set("newPassword", e.target.value)}
					onBlur={() => touch("newPassword")}
				/>
				{errors.newPassword && (
					<span className="text-destructive mt-1 block text-xs">
						{errors.newPassword}
					</span>
				)}
			</label>

			<label className="mb-3 block">
				<span className="mb-1 block text-sm">确认密码</span>
				<PasswordInput
					value={values.confirmPassword}
					placeholder="请再次输入新密码"
					onChange={(e) => set("confirmPassword", e.target.value)}
					onBlur={() => touch("confirmPassword")}
				/>
				{errors.confirmPassword && (
					<span className="text-destructive mt-1 block text-xs">
						{errors.confirmPassword}
					</span>
				)}
			</label>

			{error && (
				<p className="text-destructive mb-2 text-center text-sm">{error}</p>
			)}

			<Button type="submit" className="mt-2 w-full" disabled={submitting}>
				{submitting ? "提交中..." : submitText || "确认"}
			</Button>
		</form>
	);
}
