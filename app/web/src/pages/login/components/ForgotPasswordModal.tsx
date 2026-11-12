import {
	Button,
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
	Input,
	toast,
} from "@gospeak/components";
import { useState } from "react";
import { resetPassword } from "@/api/auth";
import PasswordInput from "@/components/common/passwordInput";
import { sendEmailCode } from "@/api/email";
import { toastActionError } from "@/utils/toast";

export interface ForgotPasswordModalProps {
	open: boolean;
	onClose: () => void;
}

export default function ForgotPasswordModal({
	open,
	onClose,
}: ForgotPasswordModalProps) {
	const [email, setEmail] = useState("");
	const [code, setCode] = useState("");
	const [newPassword, setNewPassword] = useState("");
	const [step, setStep] = useState<"email" | "code">("email");
	const [codeSending, setCodeSending] = useState(false);

	async function handleSendCode(e: React.FormEvent) {
		e.preventDefault();
		setCodeSending(true);
		try {
			await sendEmailCode({ email: email.trim(), scene: "reset_password" });
			setStep("code");
			toast.success("验证码已发送");
		} catch (error) {
			toastActionError(error, "验证码发送失败");
		} finally {
			setCodeSending(false);
		}
	}

	async function handleReset(e: React.FormEvent) {
		e.preventDefault();
		const pwd = newPassword;
		try {
			await resetPassword(email, code, pwd);
			onClose();
			toast.success("密码已重置，请登录");
		} catch (error) {
			toastActionError(error, "密码重置失败");
		}
	}

	if (!open) return null;

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>重置密码</DialogTitle>
				</DialogHeader>
				{step === "email" ? (
					<form onSubmit={handleSendCode}>
						<label className="mb-3 block">
							<span className="mb-1 block text-sm">邮箱</span>
							<Input
								type="email"
								required
								placeholder="请输入注册邮箱"
								value={email}
								onChange={(e) => setEmail(e.target.value)}
							/>
						</label>
						<Button type="submit" className="w-full" disabled={codeSending}>
							{codeSending ? "发送中..." : "发送验证码"}
						</Button>
					</form>
				) : (
					<form onSubmit={handleReset}>
						<label className="mb-3 block">
							<span className="mb-1 block text-sm">邮箱</span>
							<Input type="email" value={email} disabled />
						</label>
						<label className="mb-3 block">
							<span className="mb-1 block text-sm">验证码</span>
							<Input
								type="text"
								placeholder="请输入邮箱验证码"
								value={code}
								onChange={(e) => setCode(e.target.value)}
							/>
						</label>
						<label className="mb-3 block">
							<span className="mb-1 block text-sm">新密码</span>
							<PasswordInput
								name="newPassword"
								placeholder="请输入新密码"
								value={newPassword}
								onChange={(e) => setNewPassword(e.target.value)}
							/>
						</label>
						<Button type="submit" className="w-full">
							重置密码
						</Button>
					</form>
				)}
				<div className="flex justify-end">
					<Button variant="outline" onClick={onClose}>
						取消
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
