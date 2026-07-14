import LogIn from "lucide-solid/icons/log-in";
import { createSignal, Show } from "solid-js";

interface ProviderIconProps {
	name: string;
	/** 自定义图标 URL；加载失败时回退内置品牌图标 */
	iconUrl?: string;
	size?: number;
	class?: string;
}

function normalizeProviderName(name: string): string {
	return (name || "").trim().toLowerCase();
}

/** 常见 OAuth 提供商图标；未知 name 回退通用登录图标。 */
export default function ProviderIcon(props: ProviderIconProps) {
	const s = () => props.size ?? 24;
	const cls = () => props.class ?? "";
	const key = () => normalizeProviderName(props.name);
	const [customFailed, setCustomFailed] = createSignal(false);

	return (
		<Show
			when={props.iconUrl && !customFailed()}
			fallback={<BuiltinProviderIcon name={key()} size={s()} class={cls()} />}
		>
			<img
				src={props.iconUrl}
				alt=""
				width={s()}
				height={s()}
				class={`object-cover rounded-sm ${cls()}`}
				style={{ width: `${s()}px`, height: `${s()}px` }}
				onError={() => setCustomFailed(true)}
			/>
		</Show>
	);
}

function BuiltinProviderIcon(props: {
	name: string;
	size: number;
	class?: string;
}) {
	const s = () => props.size;
	const cls = () => props.class ?? "";

	switch (props.name) {
		case "github":
			return (
				<svg
					viewBox="0 0 24 24"
					width={s()}
					height={s()}
					fill="currentColor"
					class={`text-base-content ${cls()}`}
					aria-hidden="true"
				>
					<title>GitHub</title>
					<path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
				</svg>
			);
		case "google":
			return (
				<svg
					viewBox="0 0 24 24"
					width={s()}
					height={s()}
					class={cls()}
					aria-hidden="true"
				>
					<title>Google</title>
					<path
						fill="#4285F4"
						d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
					/>
					<path
						fill="#34A853"
						d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
					/>
					<path
						fill="#FBBC05"
						d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
					/>
					<path
						fill="#EA4335"
						d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
					/>
				</svg>
			);
		case "qq":
			return (
				<svg
					viewBox="0 0 24 24"
					width={s()}
					height={s()}
					fill="#1EBAFC"
					class={cls()}
					aria-hidden="true"
				>
					<title>QQ</title>
					<path d="M21.395 15.035a40 40 0 0 0-.803-2.264l-1.079-2.695c.001-.032.014-.562.014-.836C19.526 4.632 17.351 0 12 0S4.474 4.632 4.474 9.241c0 .274.013.804.014.836l-1.08 2.695a39 39 0 0 0-.802 2.264c-1.021 3.283-.69 4.643-.438 4.673.54.065 2.103-2.472 2.103-2.472 0 1.469.756 3.387 2.394 4.771-.612.188-1.363.479-1.845.835-.434.32-.379.646-.301.778.343.578 5.883.369 7.482.189 1.6.18 7.14.389 7.483-.189.078-.132.132-.458-.301-.778-.483-.356-1.233-.646-1.846-.836 1.637-1.384 2.393-3.302 2.393-4.771 0 0 1.563 2.537 2.103 2.472.251-.03.581-1.39-.438-4.673" />
				</svg>
			);
		case "gitlab":
			return (
				<svg
					viewBox="0 0 24 24"
					width={s()}
					height={s()}
					fill="currentColor"
					class={`text-[#FC6D26] ${cls()}`}
					aria-hidden="true"
				>
					<title>GitLab</title>
					<path d="M23.955 13.587l-1.347-4.135-2.668-8.213a.455.455 0 00-.867 0L16.406 9.45H7.594L4.927 1.239a.455.455 0 00-.867 0L1.392 9.452.045 13.587a.924.924 0 00.331 1.022L12 23.054l11.624-8.445a.92.92 0 00.331-1.022" />
				</svg>
			);
		case "discord":
			return (
				<svg
					viewBox="0 0 24 24"
					width={s()}
					height={s()}
					fill="#5865F2"
					class={cls()}
					aria-hidden="true"
				>
					<title>Discord</title>
					<path d="M20.317 4.37a19.79 19.79 0 00-4.885-1.515.074.074 0 00-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 00-5.487 0 12.64 12.64 0 00-.617-1.25.077.077 0 00-.079-.037A19.736 19.736 0 003.677 4.37a.07.07 0 00-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 00.031.057 19.9 19.9 0 005.993 3.03.078.078 0 00.084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 00-.041-.106 13.107 13.107 0 01-1.872-.892.077.077 0 01-.008-.128 10.2 10.2 0 00.372-.292.074.074 0 01.077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 01.078.01c.12.098.246.198.373.292a.077.077 0 01-.006.127 12.299 12.299 0 01-1.873.892.077.077 0 00-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 00.084.028 19.839 19.839 0 006.002-3.03.077.077 0 00.032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 00-.031-.03zM8.02 15.331c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z" />
				</svg>
			);
		case "microsoft":
		case "azure":
		case "azuread":
			return (
				<svg
					viewBox="0 0 24 24"
					width={s()}
					height={s()}
					class={cls()}
					aria-hidden="true"
				>
					<title>Microsoft</title>
					<path fill="#F25022" d="M1 1h10v10H1z" />
					<path fill="#7FBA00" d="M13 1h10v10H13z" />
					<path fill="#00A4EF" d="M1 13h10v10H1z" />
					<path fill="#FFB900" d="M13 13h10v10H13z" />
				</svg>
			);
		default:
			return (
				<div
					class={`flex items-center justify-center rounded-full bg-base-300 text-base-content ${cls()}`}
					style={{ width: `${s()}px`, height: `${s()}px` }}
					aria-hidden="true"
				>
					<LogIn size={Math.floor(s() * 0.5)} />
				</div>
			);
	}
}
