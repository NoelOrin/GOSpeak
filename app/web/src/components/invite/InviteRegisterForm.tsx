import { Button, Input } from "@gospeak/components";
import { Loader2 } from "lucide-react";
import { useState } from "react";
import PasswordInput from "@/components/common/passwordInput";

export interface InviteRegisterSubmit {
	username: string;
	password: string;
	nickname: string;
	email: string;
	emailCode: string;
}

export interface InviteRegisterFormProps {
	domainName: string;
	requiresApproval: boolean;
	submitting: boolean;
	error: string;
	emailRequired: boolean;
	codeSending: boolean;
	codeCountdown: number;
	onSubmit: (values: InviteRegisterSubmit) => void;
	onSendEmailCode: (email: string) => void;
}

export function InviteRegisterForm(props: InviteRegisterFormProps) {
	const [username, setUsername] = useState("");
	const [password, setPassword] = useState("");
	const [nickname, setNickname] = useState("");
	const [email, setEmail] = useState("");
	const [emailCode, setEmailCode] = useState("");
	const [touched, setTouched] = useState({ username: false, password: false });

	const usernameError =
		touched.username && !username.trim() ? "用户名 是必填项" : "";
	const passwordError = touched.password && !password ? "密码 是必填项" : "";

	const submitLabel = props.requiresApproval ? "注册并提交申请" : "注册并加入";

	function handleSubmit(event: React.FormEvent) {
		event.preventDefault();
		setTouched({ username: true, password: true });
		if (!username.trim() || password.length < 8) return;
		if (props.emailRequired && (!email.trim() || !emailCode.trim())) return;
		props.onSubmit({
			username: username.trim(),
			password,
			nickname: nickname.trim(),
			email: email.trim(),
			emailCode: emailCode.trim(),
		});
	}

	return (
		<form onSubmit={handleSubmit} className="flex flex-col gap-3" noValidate>
			<p className="text-muted-foreground text-sm leading-6">
				注册后
				{props.requiresApproval
					? "提交加入申请，管理员通过后即可进入"
					: "立即加入"}
				「{props.domainName}」。
			</p>

			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">
					用户名
				</span>
				<Input
					value={username}
					autoComplete="username"
					onChange={(e) => {
						setUsername(e.currentTarget.value);
						if (e.currentTarget.value.trim())
							setTouched((t) => ({ ...t, username: false }));
					}}
				/>
				{usernameError && (
					<span className="text-destructive mt-1 block text-xs">
						{usernameError}
					</span>
				)}
			</label>

			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">
					密码
				</span>
				<PasswordInput
					value={password}
					autoComplete="new-password"
					placeholder="至少 8 位"
					onChange={(e) => setPassword(e.currentTarget.value)}
				/>
				{passwordError && (
					<span className="text-destructive mt-1 block text-xs">
						{passwordError}
					</span>
				)}
			</label>

			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">
					昵称（选填）
				</span>
				<Input
					value={nickname}
					maxLength={32}
					placeholder="不填则使用用户名"
					onChange={(e) => setNickname(e.currentTarget.value)}
				/>
			</label>

			<label className="block">
				<span className="text-muted-foreground mb-1 block text-xs font-medium">
					邮箱{props.emailRequired ? "" : "（选填）"}
				</span>
				<Input
					type="email"
					value={email}
					autoComplete="email"
					onChange={(e) => setEmail(e.currentTarget.value)}
				/>
			</label>

			{email.trim() && (
				<div className="flex gap-2">
					<Input
						value={emailCode}
						placeholder="验证码"
						onChange={(e) => setEmailCode(e.currentTarget.value)}
					/>
					<Button
						type="button"
						variant="outline"
						disabled={props.codeSending || props.codeCountdown > 0}
						onClick={() => props.onSendEmailCode(email.trim())}
					>
						{props.codeSending
							? "发送中..."
							: props.codeCountdown > 0
								? `${props.codeCountdown}s 后重发`
								: "发送验证码"}
					</Button>
				</div>
			)}

			{props.error && (
				<p
					role="alert"
					className="border-destructive/50 bg-destructive/10 text-destructive rounded-sm border px-3 py-2 text-sm"
				>
					{props.error}
				</p>
			)}

			<Button type="submit" size="lg" disabled={props.submitting}>
				{props.submitting ? (
					<Loader2 className="size-4 animate-spin" />
				) : (
					submitLabel
				)}
			</Button>
		</form>
	);
}
