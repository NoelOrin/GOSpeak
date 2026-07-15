import ArrowLeft from "lucide-solid/icons/arrow-left";
import Save from "lucide-solid/icons/save";
import { Show } from "solid-js";
import FormField from "@/components/form/FormField";

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

export default function OAuthProviderForm(props: OAuthProviderFormProps) {
	const isEditing = () => props.isEditing;
	const isNative = (_name?: string) => props.isNative;
	const showFieldMapping = () => props.showFieldMapping;
	const saving = () => props.saving;
	const name = () => props.name;
	const displayName = () => props.displayName;
	const iconURL = () => props.iconURL;
	const clientID = () => props.clientID;
	const clientSecret = () => props.clientSecret;
	const authURL = () => props.authURL;
	const tokenURL = () => props.tokenURL;
	const userInfoURL = () => props.userInfoURL;
	const redirectURL = () => props.redirectURL;
	const scopes = () => props.scopes;
	const uidField = () => props.uidField;
	const usernameField = () => props.usernameField;
	const avatarField = () => props.avatarField;
	const emailField = () => props.emailField;
	const enabled = () => props.enabled;
	const setName = props.setName;
	const setDisplayName = props.setDisplayName;
	const setIconURL = props.setIconURL;
	const setClientID = props.setClientID;
	const setClientSecret = props.setClientSecret;
	const setAuthURL = props.setAuthURL;
	const setTokenURL = props.setTokenURL;
	const setUserInfoURL = props.setUserInfoURL;
	const setRedirectURL = props.setRedirectURL;
	const setScopes = props.setScopes;
	const setUidField = props.setUidField;
	const setUsernameField = props.setUsernameField;
	const setAvatarField = props.setAvatarField;
	const setEmailField = props.setEmailField;
	const setEnabled = props.setEnabled;
	const setFormMode = (mode: "preset") => {
		if (mode === "preset") props.onBackToPreset();
	};
	const handleSave = () => props.onSave();
	const closeForm = () => props.onCancel();

	return (
		<div class="card bg-base-200 shadow-sm">
			<div class="card-body gap-3">
				<div class="flex items-center gap-2">
					<Show when={!isEditing()}>
						<button
							class="btn btn-ghost btn-xs"
							onClick={() => setFormMode("preset")}
						>
							<ArrowLeft size={14} />
						</button>
					</Show>
					<h3 class="font-bold text-base">
						{isEditing() ? "编辑提供商" : `配置 ${displayName() || name()}`}
					</h3>
					<Show when={!isNative(name()) && !isEditing()}>
						<span class="badge badge-info badge-sm">通用 OAuth2</span>
					</Show>
					<Show when={isNative(name())}>
						<span class="badge badge-ghost badge-sm">内置实现</span>
					</Show>
				</div>

				<div class="divider my-0 text-xs text-base-content/40">基本信息</div>
				<div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
					<FormField label="名称（唯一标识）">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="如 my-oauth、gitlab、gitea"
							value={name()}
							onInput={(e) => setName(e.currentTarget.value)}
							disabled={isEditing()}
						/>
					</FormField>
					<FormField label="显示名称">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="登录页显示的名称"
							value={displayName()}
							onInput={(e) => setDisplayName(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="图标 URL">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="https://example.com/icon.svg"
							value={iconURL()}
							onInput={(e) => setIconURL(e.currentTarget.value)}
						/>
					</FormField>
				</div>

				<div class="divider my-0 text-xs text-base-content/40">OAuth 端点</div>
				<div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
					<FormField label="Client ID">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="OAuth App Client ID"
							value={clientID()}
							onInput={(e) => setClientID(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="Client Secret">
						<input
							type="password"
							class="input input-bordered input-sm w-full"
							placeholder={
								isEditing() ? "留空保持原值" : "OAuth App Client Secret"
							}
							value={clientSecret()}
							onInput={(e) => setClientSecret(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="授权 URL (Auth URL)">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="https://example.com/oauth/authorize"
							value={authURL()}
							onInput={(e) => setAuthURL(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="Token URL">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="https://example.com/oauth/token"
							value={tokenURL()}
							onInput={(e) => setTokenURL(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="UserInfo URL">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="https://example.com/api/user"
							value={userInfoURL()}
							onInput={(e) => setUserInfoURL(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="回调 URL (Redirect URL)">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="https://your-domain/api/v1/oauth/callback/my-oauth"
							value={redirectURL()}
							onInput={(e) => setRedirectURL(e.currentTarget.value)}
						/>
					</FormField>
					<FormField label="Scopes">
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="openid profile email"
							value={scopes()}
							onInput={(e) => setScopes(e.currentTarget.value)}
						/>
					</FormField>
				</div>

				<Show when={showFieldMapping()}>
					<div class="divider my-0 text-xs text-base-content/40">
						用户信息字段映射
					</div>
					<p class="text-xs text-base-content/50">
						指定 UserInfo API 返回 JSON 中各字段的 key 名，用于解析用户信息
					</p>
					<div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
						<FormField label="UID 字段名">
							<input
								type="text"
								class="input input-bordered input-sm w-full"
								placeholder="id（默认）"
								value={uidField()}
								onInput={(e) => setUidField(e.currentTarget.value)}
							/>
						</FormField>
						<FormField label="用户名字段名">
							<input
								type="text"
								class="input input-bordered input-sm w-full"
								placeholder="username（默认）"
								value={usernameField()}
								onInput={(e) => setUsernameField(e.currentTarget.value)}
							/>
						</FormField>
						<FormField label="头像字段名">
							<input
								type="text"
								class="input input-bordered input-sm w-full"
								placeholder="avatar_url（默认）"
								value={avatarField()}
								onInput={(e) => setAvatarField(e.currentTarget.value)}
							/>
						</FormField>
						<FormField label="邮箱字段名">
							<input
								type="text"
								class="input input-bordered input-sm w-full"
								placeholder="email（默认）"
								value={emailField()}
								onInput={(e) => setEmailField(e.currentTarget.value)}
							/>
						</FormField>
					</div>
				</Show>

				<div class="flex items-center justify-between py-2">
					<span class="text-sm font-medium">启用</span>
					<input
						type="checkbox"
						class="toggle toggle-sm toggle-primary"
						checked={enabled()}
						onChange={(e) => setEnabled(e.currentTarget.checked)}
					/>
				</div>

				<div class="flex gap-2">
					<button
						type="button"
						class="btn btn-primary btn-sm"
						classList={{ "btn-disabled": saving() }}
						onClick={handleSave}
					>
						<Show when={saving()} fallback={<Save size={16} />}>
							<span class="loading loading-spinner loading-xs" />
						</Show>
						{saving() ? "保存中..." : "保存"}
					</button>
					<button
						type="button"
						class="btn btn-ghost btn-sm"
						onClick={closeForm}
					>
						取消
					</button>
				</div>
			</div>
		</div>
	);
}
