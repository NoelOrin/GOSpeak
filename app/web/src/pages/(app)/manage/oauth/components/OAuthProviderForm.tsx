import { Button, Switch } from "@gospeak/components";
import { ArrowLeft, Save } from "lucide-react";
import type { ReactNode } from "react";
import PasswordInput from "@/components/common/passwordInput";

export interface OAuthProviderFormProps {
	isEditing: boolean;
	isNative: boolean;
	showFieldMapping: boolean;
	saving: boolean;
	name: string;
	displayName: string;
	iconURL: string;
	clientID: string;
	clientSecret: string;
	authURL: string;
	tokenURL: string;
	userInfoURL: string;
	redirectURL: string;
	scopes: string;
	uidField: string;
	usernameField: string;
	avatarField: string;
	emailField: string;
	enabled: boolean;
	setName: (v: string) => void;
	setDisplayName: (v: string) => void;
	setIconURL: (v: string) => void;
	setClientID: (v: string) => void;
	setClientSecret: (v: string) => void;
	setAuthURL: (v: string) => void;
	setTokenURL: (v: string) => void;
	setUserInfoURL: (v: string) => void;
	setRedirectURL: (v: string) => void;
	setScopes: (v: string) => void;
	setUidField: (v: string) => void;
	setUsernameField: (v: string) => void;
	setAvatarField: (v: string) => void;
	setEmailField: (v: string) => void;
	setEnabled: (v: boolean) => void;
	onBackToPreset: () => void;
	onSave: () => void;
	onCancel: () => void;
}

const inputClass =
	"border-input bg-card text-foreground focus-visible:ring-ring placeholder:text-faint-foreground flex h-9 w-full rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50";

function FormField({
	label,
	children,
}: {
	label: string;
	children: ReactNode;
}) {
	return (
		<label className="block">
			<span className="mb-1 block text-sm">{label}</span>
			{children}
		</label>
	);
}

function Tag({ children }: { children: ReactNode }) {
	return (
		<span className="border-border bg-card text-foreground inline-flex items-center rounded-full border px-2 py-0.5 text-2xs font-medium">
			{children}
		</span>
	);
}

export default function OAuthProviderForm(props: OAuthProviderFormProps) {
	return (
		<section className="border-border/80 bg-card rounded-lg border p-4 shadow-sm md:p-5">
			{/* 用 form 承接 Enter 提交：比在 div 上挂 keydown 更符合原生语义，
			     也不会抢走 Switch / 按钮自己的键盘行为 */}
			<form
				className="flex flex-col gap-3"
				onSubmit={(e) => {
					e.preventDefault();
					if (!props.saving) props.onSave();
				}}
			>
				<div className="flex items-center gap-2">
					{!props.isEditing && (
						<button
							type="button"
							className="hover:bg-muted inline-flex size-7 cursor-pointer items-center justify-center rounded-sm transition-colors"
							onClick={props.onBackToPreset}
						>
							<ArrowLeft size={14} />
						</button>
					)}
					<h3 className="text-base font-bold">
						{props.isEditing
							? "编辑提供商"
							: `配置 ${props.displayName || props.name}`}
					</h3>
					{!props.isNative && !props.isEditing && <Tag>通用 OAuth2</Tag>}
					{props.isNative && <Tag>内置实现</Tag>}
				</div>

				<div className="text-faint-foreground mt-1 text-xs font-semibold">
					基本信息
				</div>
				<div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
					<FormField label="名称（唯一标识）">
						<input
							type="text"
							className={inputClass}
							placeholder="如 my-oauth、gitlab、gitea"
							value={props.name}
							onChange={(e) => props.setName(e.currentTarget.value)}
							disabled={props.isEditing}
						/>
					</FormField>
					<FormField label="显示名称">
						<input
							type="text"
							className={inputClass}
							placeholder="登录页显示的名称"
							value={props.displayName}
							onChange={(e) => props.setDisplayName(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="图标 URL">
						<input
							type="text"
							className={inputClass}
							placeholder="https://example.com/icon.svg"
							value={props.iconURL}
							onChange={(e) => props.setIconURL(e.currentTarget.value)}
						/>
					</FormField>
				</div>

				<div className="text-faint-foreground mt-1 text-xs font-semibold">
					OAuth 端点
				</div>
				<div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
					<FormField label="Client ID">
						<input
							type="text"
							className={inputClass}
							placeholder="OAuth App Client ID"
							value={props.clientID}
							onChange={(e) => props.setClientID(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="Client Secret">
						<PasswordInput
							native
							autoComplete="off"
							className={inputClass}
							placeholder={
								props.isEditing ? "留空保持原值" : "OAuth App Client Secret"
							}
							value={props.clientSecret}
							onChange={(e) => props.setClientSecret(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="授权 URL (Auth URL)">
						<input
							type="text"
							className={inputClass}
							placeholder="https://example.com/oauth/authorize"
							value={props.authURL}
							onChange={(e) => props.setAuthURL(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="Token URL">
						<input
							type="text"
							className={inputClass}
							placeholder="https://example.com/oauth/token"
							value={props.tokenURL}
							onChange={(e) => props.setTokenURL(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="UserInfo URL">
						<input
							type="text"
							className={inputClass}
							placeholder="https://example.com/api/user"
							value={props.userInfoURL}
							onChange={(e) => props.setUserInfoURL(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="回调 URL (Redirect URL)">
						<input
							type="text"
							className={inputClass}
							placeholder="https://your-domain/api/v1/oauth/callback/my-oauth"
							value={props.redirectURL}
							onChange={(e) => props.setRedirectURL(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="Scopes">
						<input
							type="text"
							className={inputClass}
							placeholder="openid profile email"
							value={props.scopes}
							onChange={(e) => props.setScopes(e.currentTarget.value)}
						/>
					</FormField>
				</div>

				{props.showFieldMapping && (
					<>
						<div className="text-faint-foreground mt-1 text-xs font-semibold">
							用户信息字段映射
						</div>
						<p className="text-faint-foreground text-xs">
							指定 UserInfo API 返回 JSON 中各字段的 key 名，用于解析用户信息
						</p>
						<div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
							<FormField label="UID 字段名">
								<input
									type="text"
									className={inputClass}
									placeholder="id（默认）"
									value={props.uidField}
									onChange={(e) => props.setUidField(e.currentTarget.value)}
								/>
							</FormField>
							<FormField label="用户名字段名">
								<input
									type="text"
									className={inputClass}
									placeholder="username（默认）"
									value={props.usernameField}
									onChange={(e) =>
										props.setUsernameField(e.currentTarget.value)
									}
								/>
							</FormField>
							<FormField label="头像字段名">
								<input
									type="text"
									className={inputClass}
									placeholder="avatar_url（默认）"
									value={props.avatarField}
									onChange={(e) => props.setAvatarField(e.currentTarget.value)}
								/>
							</FormField>
							<FormField label="邮箱字段名">
								<input
									type="text"
									className={inputClass}
									placeholder="email（默认）"
									value={props.emailField}
									onChange={(e) => props.setEmailField(e.currentTarget.value)}
								/>
							</FormField>
						</div>
					</>
				)}

				<div className="flex items-center justify-between py-2">
					<span className="text-sm font-medium">启用</span>
					<Switch
						checked={props.enabled}
						onCheckedChange={props.setEnabled}
						aria-label="启用"
					/>
				</div>

				<div className="flex gap-2">
					<Button type="submit" variant="outline" disabled={props.saving}>
						<Save size={16} />
						{props.saving ? "保存中..." : "保存"}
					</Button>
					<Button type="button" variant="ghost" onClick={props.onCancel}>
						取消
					</Button>
				</div>
			</form>
		</section>
	);
}
