import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import clsx from "clsx";
import { Mic, MicOff } from "lucide-react";
import { createOverlay } from "@gospeak/desktop";
import { useUserStore } from "@/stores/userStore";
import { overlayStore, useOverlayStore } from "@/stores/overlayStore";

// 文字浮在游戏画面上，没有卡片背景兜底，统一用白色 + 深色描影保证可读。
const TEXT_SHADOW = "[text-shadow:0_1px_2px_rgba(0,0,0,0.9)]";

function OverlayHud() {
	const user = useUserStore((s) => s.user);
	const members = useOverlayStore((s) => s.members);
	const speakingIdentities = useOverlayStore((s) => s.speakingIdentities);
	// 默认锁定：锁定时不拦截游戏鼠标，也不可拖拽；快捷键解锁后 HUD 区域
	// 变为 --app-region: drag，可整体拖到任意位置。
	const [locked, setLocked] = useState(true);

	// 启动即从宿主拉取最近一次快照，并订阅后续推送（来自主窗口接力）。
	useEffect(() => {
		const overlay = createOverlay();
		let unsubState = () => {};
		let unsubLock = () => {};
		void overlay.getState().then((snap) => {
			if (snap) overlayStore.set(snap);
			unsubState = overlay.onState((s) => overlayStore.set(s));
		});
		void overlay.getLocked().then(setLocked);
		unsubLock = overlay.onLockChange(setLocked);
		return () => {
			unsubState();
			unsubLock();
		};
	}, []);

	const speaking = new Set(speakingIdentities);
	const displayNameOf = (m: (typeof members)[number]) => {
		const isMe = !!user && m.identity === user.name;
		if (isMe)
			return user?.display_name || m.displayName || m.name || m.identity;
		return m.displayName || m.name || m.identity;
	};

	// 发言人排前面，其余保持原顺序，便于一眼看到"谁在说话"。
	const ordered = [...members].sort((a, b) => {
		const sa = speaking.has(a.identity) ? 0 : 1;
		const sb = speaking.has(b.identity) ? 0 : 1;
		return sa - sb;
	});

	// 只渲染成员列表：没有标题、房间名、连接状态、底部统计——
	// 成员为空时页面完全不画内容（窗口纯透明，等于不可见）。
	if (ordered.length === 0) {
		return <div className="pointer-events-none fixed inset-0" />;
	}

	return (
		<div className="pointer-events-none fixed inset-0">
			<ul
				className={clsx(
					"fixed top-4 left-4 w-52 flex-col gap-0.5 select-none",
					locked
						? "pointer-events-none"
						: "pointer-events-auto [--app-region:drag]",
				)}
			>
				{ordered.map((m) => {
					const isSpeaking = speaking.has(m.identity);
					const isMe = !!user && m.identity === user.name;
					return (
						// 解锁状态下整块 HUD 是拖拽区（--app-region: drag），
						// 行内不注册任何点击行为，拖动即移动窗口。
						<li
							key={m.identity}
							className={clsx(
								"flex items-center gap-1.5 rounded px-1 py-0.5",
								TEXT_SHADOW,
							)}
						>
							<span
								className={clsx(
									"flex size-4 shrink-0 items-center justify-center",
									isSpeaking ? "text-emerald-300" : "text-white/45",
								)}
							>
								{m.isMicMuted ? <MicOff size={13} /> : <Mic size={13} />}
							</span>
							<span
								className={clsx(
									"min-w-0 flex-1 truncate text-xs",
									isSpeaking ? "font-semibold text-white" : "text-white/70",
								)}
							>
								{displayNameOf(m)}
								{isMe && (
									<span className="text-white/45 ml-1 text-2xs">(你)</span>
								)}
							</span>
						</li>
					);
				})}
			</ul>
		</div>
	);
}

/**
 * 平台隔离：覆盖层只在桌面端宿主里作为浮动窗口使用。
 * `__GOSPEAK_DESKTOP__` 是编译期常量：Web 构建中恒 false，HUD 分支整体
 * 折叠剔除（OverlayHud 的桌面能力依赖不进 Web 包）；桌面构建恒 true。
 * 拆成外层纯判断 + 内层 HUD，避免 useEffect 出现在提前 return 之后
 * （hooks 必须每次渲染都以相同顺序调用）。
 */
function OverlayPage() {
	if (!__GOSPEAK_DESKTOP__) {
		return (
			<div className="bg-background text-foreground flex min-h-dvh flex-col items-center justify-center gap-2 p-6 text-center">
				<span className="text-primary text-xl font-semibold">GOSpeak</span>
				<p className="text-muted-foreground text-sm">
					游戏内覆盖层仅在 GOSpeak 桌面端中可用。
				</p>
			</div>
		);
	}
	return <OverlayHud />;
}

export const Route = createFileRoute("/overlay")({
	component: OverlayPage,
});
