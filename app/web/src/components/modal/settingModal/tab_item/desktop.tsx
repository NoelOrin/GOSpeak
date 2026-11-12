import {
	Button,
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@gospeak/components";
import type {
	OverlayConfig,
	OverlayPolicy,
	UpdateInfo,
} from "@gospeak/desktop";
import { Field, Page, Section, Toggle } from "./shared";
import ServerUrlField from "@/components/common/serverUrlField";
import { createDesktop } from "@gospeak/desktop";
import { useEffect, useState } from "react";
import type { SettingTabConfig } from "./types";

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

type UpdateStatus =
	| "idle"
	| "checking"
	| "latest"
	| "available"
	| "downloading"
	| "restarting"
	| "unsupported"
	| "error";

function UpdateSection() {
	const desktop = createDesktop();

	const [current, setCurrent] = useState("");
	const [status, setStatus] = useState<UpdateStatus>("idle");
	const [update, setUpdate] = useState<UpdateInfo | null>(null);
	const [progress, setProgress] = useState(0);
	const [error, setError] = useState("");

	// 下载进度事件常驻订阅：Install 期间宿主按已下载/总量广播 0..1。
	// biome-ignore lint/correctness/useExhaustiveDependencies: 仅在 desktop.isDesktop 变化时订阅更新进度，desktop 门面对象稳定
	useEffect(() => {
		if (!desktop.isDesktop) return;
		return desktop.updater.onProgress(setProgress);
	}, [desktop.isDesktop]);

	const check = async () => {
		setStatus("checking");
		setError("");
		setUpdate(null);
		try {
			const info = await desktop.updater.check();
			setCurrent(info.current);
			if (!info.supported) {
				setStatus("unsupported");
			} else if (info.available) {
				setUpdate(info);
				setStatus("available");
			} else {
				setStatus("latest");
			}
		} catch (e) {
			setStatus("error");
			setError(e instanceof Error ? e.message : String(e));
		}
	};

	const install = async () => {
		setStatus("downloading");
		setProgress(0);
		try {
			await desktop.updater.install();
			// 安装成功后宿主重启进新版本，页面随之关闭；能走到这里说明重启未发生。
			setStatus("restarting");
		} catch (e) {
			setStatus("error");
			setError(e instanceof Error ? e.message : String(e));
		}
	};

	// 纯浏览器里没有桌面宿主，本区块不渲染（避免误导性的兜底状态）。
	if (!desktop.isDesktop) return null;

	return (
		<Section
			title="软件更新"
			desc="检查 GitHub Releases 上的新版本；安装会校验签名并替换当前应用后重启。"
		>
			<div className="flex flex-col gap-2">
				<div className="flex items-center justify-between gap-4">
					<div className="min-w-0">
						<div className="text-sm font-medium">
							当前版本
							<span className="text-faint-foreground ml-2 font-normal">
								{current || "…"}
							</span>
						</div>
						{status === "checking" && (
							<div className="text-faint-foreground text-xs">正在检查更新…</div>
						)}
						{status === "latest" && <div className="text-xs">已是最新版本</div>}
						{status === "unsupported" && (
							<div className="text-faint-foreground text-xs">
								当前构建不支持自动更新，请前往发布页手动下载。
							</div>
						)}
						{status === "downloading" && (
							<div className="text-faint-foreground text-xs">
								正在下载更新… {Math.round(progress * 100)}%
							</div>
						)}
						{status === "restarting" && (
							<div className="text-xs">更新完成，正在重启…</div>
						)}
						{status === "error" && (
							<div className="text-destructive text-xs" title={error}>
								检查/安装失败：{error}
							</div>
						)}
					</div>
					<div className="flex shrink-0 items-center gap-2">
						{status === "available" ? (
							<Button size="sm" onClick={() => void install()}>
								下载并安装
							</Button>
						) : status === "downloading" ? null : (
							<Button
								size="sm"
								variant="outline"
								disabled={status === "checking"}
								onClick={() => void check()}
							>
								检查更新
							</Button>
						)}
						{status === "unsupported" && (
							<Button
								size="sm"
								onClick={() => void desktop.updater.openReleasePage()}
							>
								前往发布页
							</Button>
						)}
					</div>
				</div>

				{status === "downloading" && (
					<div className="bg-border h-1.5 w-full overflow-hidden rounded-sm">
						<div
							className="bg-primary h-full transition-[width]"
							style={{ width: `${Math.round(progress * 100)}%` }}
						/>
					</div>
				)}

				{status === "available" && update && (
					<div className="border-border bg-muted/40 rounded-sm border px-3 py-2">
						<div className="text-sm">
							发现新版本
							<span className="ml-2 font-medium">{update.version}</span>
						</div>
						{update.notes ? (
							<div className="text-faint-foreground mt-1 max-h-32 overflow-y-auto whitespace-pre-wrap text-xs">
								{update.notes}
							</div>
						) : null}
					</div>
				)}
			</div>
		</Section>
	);
}

function DesktopForm() {
	const desktop = createDesktop();

	const [cfg, setCfg] = useState<OverlayConfig | null>(null);
	const [autostartEnabled, setAutostartEnabled] = useState(false);

	// biome-ignore lint/correctness/useExhaustiveDependencies: 挂载时读取一次自启动与覆盖层配置，后续由 patch 显式刷新，不应随每次渲染重读
	useEffect(() => {
		void desktop.overlay
			.getConfig()
			.then(setCfg)
			.catch(() => {});
		void desktop.autostart
			.isEnabled()
			.then(setAutostartEnabled)
			.catch(() => {});
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

			<UpdateSection />

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
