import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@gospeak/components";
import type { OverlayConfig, OverlayPolicy } from "@gospeak/desktop";
import { Field, Page, Section, Toggle } from "./shared";
import ServerUrlField from "@/components/common/serverUrlField";
import { createDesktop } from "@gospeak/desktop";
import { useEffect, useState } from "react";
import type { SettingTabConfig } from "./types";

const POLICY_OPTIONS: { value: OverlayPolicy; label: string; desc: string }[] =
	[
		{
			value: "auto",
			label: "自动（全屏 + 通话中）",
			desc: "仅当前台有全屏应用且已进入语音房间连接时显示；快捷键可临时强制显隐",
		},
		{
			value: "manual",
			label: "手动",
			desc: "只由快捷键或托盘控制显隐",
		},
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

function DesktopForm() {
	const desktop = createDesktop();

	const [cfg, setCfg] = useState<OverlayConfig | null>(null);
	const [autostartEnabled, setAutostartEnabled] = useState(false);

	useEffect(() => {
		void desktop.overlay
			.getConfig()
			.then(setCfg)
			.catch(() => {});
		void desktop.autostart
			.isEnabled()
			.then(setAutostartEnabled)
			.catch(() => {});
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, []);

	const patch = (partial: Partial<OverlayConfig>) =>
		setCfg((prev) => (prev ? { ...prev, ...partial } : prev));

	return (
		<Page
			title="桌面端"
			desc="仅在 GOSpeak 桌面端（MyGo 宿主）中可用。游戏内覆盖层、开机自启等原生能力。"
		>
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
						value={cfg?.policy ?? "manual"}
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
						自动模式：全屏应用台前 +
						语音房间已连接时显示，任一条件消失自动隐藏；
						显隐快捷键的最后一次操作优先生效，直到条件再次变化。
					</p>
				)}

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
			</Section>

			<Section title="启动与常驻">
				<Toggle
					label="开机自动启动"
					desc="登录系统后自动打开 GOSpeak"
					checked={autostartEnabled}
					onChange={(v) => {
						setAutostartEnabled(v);
						if (v) void desktop.autostart.enable();
						else void desktop.autostart.disable();
					}}
				/>
			</Section>

			<Section
				title="服务器"
				desc="桌面端连接的后端地址，持久化保存在本机；保存后自动连接并刷新。"
			>
				<ServerUrlField />
			</Section>

			<div className="border-info/30 bg-info/10 text-info flex items-center rounded-sm px-3 py-2 text-xs">
				提示：原生通知、系统托盘、文件对话框等桌面能力也通过同一套接口提供，由宿主（MyGo）实现。
			</div>
		</Page>
	);
}

const desktopTab: SettingTabConfig = {
	id: "desktop",
	label: "桌面端",
	icon: "monitor",
	component: DesktopForm,
};

export default desktopTab;
