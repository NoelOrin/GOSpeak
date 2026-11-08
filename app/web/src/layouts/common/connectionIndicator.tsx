import { socketStore, useSocketStore } from "@/stores/socketStore";

/**
 * 顶栏常驻连接指示。
 *
 * socketStore 里本来就有 connected / connecting，但此前只有房间列表区在消费，
 * 进入管理页 / 资料页 / 文字房后用户完全不知道自己还连着没有。
 * 点一下可以在断开时手动重连。
 */
export default function ConnectionIndicator() {
	const connected = useSocketStore((s) => s.connected);
	const connecting = useSocketStore((s) => s.connecting);

	const state = connected ? "online" : connecting ? "connecting" : "offline";
	const label =
		state === "online"
			? "已连接"
			: state === "connecting"
				? "连接中"
				: "已断开";
	const dot =
		state === "online"
			? "bg-success"
			: state === "connecting"
				? "bg-warning animate-pulse"
				: "bg-destructive";
	const text =
		state === "online"
			? "text-faint-foreground"
			: state === "connecting"
				? "text-warning"
				: "text-destructive";

	return (
		<button
			type="button"
			className={`flex cursor-pointer items-center gap-1.5 rounded-sm px-2 py-1 text-xs transition-colors ${
				state === "offline" ? "hover:bg-muted" : "cursor-default"
			}`}
			title={state === "offline" ? "连接已断开，点击重新连接" : label}
			aria-label={`服务器${label}`}
			onClick={() => {
				if (state === "offline") socketStore.connect();
			}}
			disabled={state !== "offline"}
		>
			<span className={`size-1.5 shrink-0 rounded-full ${dot}`} />
			<span className={text}>{label}</span>
		</button>
	);
}
