import { createDesktop } from "@gospeak/desktop";
import { useEffect } from "react";
import { useThemeStore } from "@/stores/themeStore";
import { useUserStore } from "@/stores/userStore";
import AppTitleBar from "./appTitleBar";
import { useRouteTitle } from "./useRouteTitle";

/**
 * 桌面构建的窗口标题栏。Web 构建里 `__GOSPEAK_DESKTOP__` 恒为 false，
 * 组件返回 null，DOM 与改动前一致。
 */
export default function DesktopTitleBar() {
	if (!__GOSPEAK_DESKTOP__) return null;
	return <DesktopTitleBarLive />;
}

function DesktopTitleBarLive() {
	const title = useRouteTitle();
	const isDark = useThemeStore((s) => s.isDark);
	const loggedIn = useUserStore((s) => !!s.user);

	// 原生外观（Windows 标题按钮字形、macOS / Linux 的菜单与对话框）跟随应用主题。
	useEffect(() => {
		void createDesktop()
			.window.setAppearance(isDark ? "dark" : "light")
			.catch(() => {});
	}, [isDark]);

	return <AppTitleBar title={title} showConnection={loggedIn} />;
}
