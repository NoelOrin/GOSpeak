export const NATIVE_PROVIDERS = new Set(["github", "google", "qq"]);

// 常用 OAuth 提供商预设模板
export interface ProviderPreset {
	name: string;
	label: string;
	auth_url: string;
	token_url: string;
	user_info_url: string;
	scopes: string;
	uid_field: string;
	username_field: string;
	avatar_field: string;
	email_field: string;
}

export const PROVIDER_PRESETS: ProviderPreset[] = [
	{
		name: "github",
		label: "GitHub",
		auth_url: "https://github.com/login/oauth/authorize",
		token_url: "https://github.com/login/oauth/access_token",
		user_info_url: "https://api.github.com/user",
		scopes: "read:user user:email",
		uid_field: "id",
		username_field: "login",
		avatar_field: "avatar_url",
		email_field: "email",
	},
	{
		name: "google",
		label: "Google",
		auth_url: "https://accounts.google.com/o/oauth2/v2/auth",
		token_url: "https://oauth2.googleapis.com/token",
		user_info_url: "https://www.googleapis.com/oauth2/v3/userinfo",
		scopes: "openid email profile",
		uid_field: "sub",
		username_field: "name",
		avatar_field: "picture",
		email_field: "email",
	},
	{
		name: "qq",
		label: "QQ",
		auth_url: "https://graph.qq.com/oauth2.0/authorize",
		token_url: "https://graph.qq.com/oauth2.0/token",
		user_info_url: "https://graph.qq.com/user/get_user_info",
		scopes: "get_user_info",
		uid_field: "openid",
		username_field: "nickname",
		avatar_field: "figureurl_qq_2",
		email_field: "",
	},
	{
		name: "gitlab",
		label: "GitLab",
		auth_url: "https://gitlab.com/oauth/authorize",
		token_url: "https://gitlab.com/oauth/token",
		user_info_url: "https://gitlab.com/api/v4/user",
		scopes: "read_user",
		uid_field: "id",
		username_field: "username",
		avatar_field: "avatar_url",
		email_field: "email",
	},
	{
		name: "discord",
		label: "Discord",
		auth_url: "https://discord.com/api/oauth2/authorize",
		token_url: "https://discord.com/api/oauth2/token",
		user_info_url: "https://discord.com/api/users/@me",
		scopes: "identify email",
		uid_field: "id",
		username_field: "username",
		avatar_field: "avatar",
		email_field: "email",
	},
	{
		name: "microsoft",
		label: "Microsoft",
		auth_url: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		token_url: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		user_info_url: "https://graph.microsoft.com/oidc/userinfo",
		scopes: "openid email profile",
		uid_field: "sub",
		username_field: "name",
		avatar_field: "picture",
		email_field: "email",
	},
];
