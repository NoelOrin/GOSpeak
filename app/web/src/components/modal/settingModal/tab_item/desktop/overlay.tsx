import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@gospeak/components";
import type { OverlayConfig, OverlayPolicy } from "@gospeak/desktop";
import { Field, Section, Toggle } from "../shared";
import { createDesktop } from "@gospeak/desktop";
import { useEffect, useState } from "react";

const POLICY_OPTIONS: { value: OverlayPolicy; label: string; desc: string }[] =
	[
		{
			value: "auto",
			label: "自动（离开 GOSpeak 时显示）",
			desc: "已进入语音房间且 GOSpeak 不在前台时显示；切回 GOSpeak 自动隐藏",
		},
		{
			value: "always",
			label: "始终显示",
			desc: "只要在语音房间里就一直显示，适合把游戏放在另一块屏幕",
		},
		{
			value: "manual",
			label: "手动",
			desc: "只由快捷键或托盘控制显隐",
		},
	];

const SCALE_OPTIONS: { value: number; label: string }[] = [
	{ value: 0, label: "自动（按显示器）" },
	{ value: 0.8, label: "80%" },
	{ value: 1, label: "100%" },
	{ value: 1.25, label: "125%" },
	{ value: 1.5, label: "150%" },
];

function HotkeyField({
	value,
	placeholder,
	onCommit,
}: {
	value: string;
	placeholder: string;
	onCommit: (v: string) => void;
}) {
	const [draft, setDraft] = useState(value);
	// 外部值变化（如宿主加载完成）时同步草稿
	useEffect(() => setDraft(value), [value]);

	return (
		<input
			type="text"
			value={draft}
			placeholder={placeholder}
			className="border-border bg-background h-9 w-full rounded-sm border px-2 text-sm"
			onChange={(e) => setDraft(e.target.value)}
			// 快捷键在失焦 / 回车时才注册：避免每敲一个字符就抢占一次全局热键
			onBlur={() => {
				const v = draft.trim();
				if (v !== value) onCommit(v);
			}}
			onKeyDown={(e) => {
				if (e.key === "Enter") (e.target as HTMLInputElement).blur();
			}}
		/>
	);
}

export function OverlaySection({
	cfg,
	patch,
}: {
	cfg: OverlayConfig | null;
	patch: (partial: Partial<OverlayConfig>) => void;
}) {
	const desktop = createDesktop();

	return (
		<Section
			title="游戏内覆盖层"
			desc="无边框、透明、无阴影的置顶浮动层，只显示房间成员。默认关闭。"
		>
			<Toggle
				label="启用游戏内覆盖层"
				desc="总开关；显示时机由下方策略决定"
				checked={cfg?.enabled ?? false}
				onChange={(v) => {
					patch({ enabled: v });
					void desktop.overlay.setEnabled(v);
				}}
			/>

			<Field label="显示策略">
				<Select
					value={cfg?.policy ?? "auto"}
					onValueChange={(value) => {
						patch({ policy: value as OverlayPolicy });
						void desktop.overlay.setPolicy(value as OverlayPolicy);
					}}
				>
					<SelectTrigger>
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{POLICY_OPTIONS.map((o) => (
							<SelectItem key={o.value} value={o.value}>
								{o.label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			</Field>
			{cfg?.policy === "auto" && (
				<p className="text-faint-foreground text-xs">
					自动模式：进入语音房间后，切到其他应用时显示，切回 GOSpeak
					自动隐藏；显隐快捷键的最后一次操作优先生效，直到条件再次变化。
				</p>
			)}

			<Field label="HUD 大小" desc="自动会按显示器分辨率选择合适大小">
				<Select
					value={String(cfg?.scale ?? 0)}
					onValueChange={(value) => {
						const v = Number(value);
						patch({ scale: v });
						void desktop.overlay.setScale(v);
					}}
				>
					<SelectTrigger>
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{SCALE_OPTIONS.map((o) => (
							<SelectItem key={o.value} value={String(o.value)}>
								{o.label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			</Field>

			<Toggle
				label="防录屏"
				desc="开启后直播、录屏与截图看不到覆盖层；仅 macOS 与 Windows 生效"
				checked={cfg?.contentProtection ?? false}
				onChange={(v) => {
					patch({ contentProtection: v });
					void desktop.overlay.setContentProtection(v);
				}}
			/>

			<Field
				label="不透明度"
				hint={`${Math.round((cfg?.opacity ?? 1) * 100)}%`}
			>
				<input
					type="range"
					min={0.2}
					max={1}
					step={0.05}
					value={cfg?.opacity ?? 1}
					className="bg-border h-1.5 w-full cursor-pointer accent-[var(--primary)]"
					onChange={(e) => {
						const v = Number(e.target.value);
						patch({ opacity: v });
						void desktop.overlay.setOpacity(v);
					}}
				/>
			</Field>

			<Field
				label="显隐快捷键"
				desc="全局快捷键，一键显示/隐藏覆盖层（如 Ctrl+Shift+O；失焦后生效）"
			>
				<HotkeyField
					value={cfg?.hotkey ?? ""}
					placeholder="Ctrl+Shift+O"
					onCommit={(v) => {
						patch({ hotkey: v });
						void desktop.overlay.setHotkey(v);
					}}
				/>
			</Field>

			<Field
				label="锁定/解锁位置快捷键"
				desc="解锁后可拖动覆盖层到任意位置，锁定后不拦截游戏鼠标（失焦后生效）"
			>
				<HotkeyField
					value={cfg?.lockHotkey ?? ""}
					placeholder="Ctrl+Shift+L"
					onCommit={(v) => {
						patch({ lockHotkey: v });
						void desktop.overlay.setLockHotkey(v);
					}}
				/>
			</Field>

			<p className="text-faint-foreground text-xs">
				若覆盖层未显示，请把游戏的显示模式改为「无边框窗口」—— Windows
				的独占全屏无法被任何置顶窗口覆盖。
			</p>
		</Section>
	);
}
