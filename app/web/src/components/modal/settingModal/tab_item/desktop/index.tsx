import type { OverlayConfig } from "@gospeak/desktop";
import { Page, Section, Toggle } from "../shared";
import ServerUrlField from "@/components/common/serverUrlField";
import { OverlaySection } from "./overlay";
import { createDesktop } from "@gospeak/desktop";
import { useEffect, useState } from "react";
import type { SettingTabConfig } from "../types";

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
			desc="仅在 GOSpeak 桌面端（MyGo 宿主）中可用。游戏内覆盖层、开机自启等原生能力；版本与更新已移至「关于」。"
		>
			<OverlaySection cfg={cfg} patch={patch} />

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
