import { Moon, Sun } from "lucide-react";
import ConnectionIndicator from "@/layouts/common/connectionIndicator";
import { toggleDark, useThemeStore } from "@/stores/themeStore";

interface AppTitleBarProps {
	title: string;
	/** 已登录页面才显示连接状态。 */
	showConnection?: boolean;
}

/**
 * 窗口标题栏。整条栏是窗口拖拽区（MyGo 的 --app-region: drag），
 * 左右内边距吃 MyGo 注入的 --mygo-titlebar-inset-*，给系统窗口按钮让位：
 * macOS 红绿灯在左、Windows / Linux 按钮在右、全屏时归零，都不需要平台分支。
 */
export default function AppTitleBar({
	title,
	showConnection = false,
}: AppTitleBarProps) {
	const isDark = useThemeStore((s) => s.isDark);

	return (
		<div
			className="app-titlebar bg-background text-foreground relative flex shrink-0 select-none items-center gap-2 overflow-hidden text-xs [--app-region:drag]"
			style={{
				height: "var(--mygo-titlebar-height, 28px)",
				// 系统按钮的 inset 之外再留 12px，字标不贴红绿灯 / Windows 按钮
				paddingLeft: "calc(var(--mygo-titlebar-inset-left, 0px) + 12px)",
				paddingRight: "calc(var(--mygo-titlebar-inset-right, 0px) + 12px)",
				boxShadow: "inset 0 -1px 0 var(--border)",
			}}
		>
			<span className="shrink-0 text-xs font-semibold tracking-wide">
				GO<span className="text-primary">Speak</span>
			</span>
			<div className="pointer-events-none absolute inset-0 flex items-center justify-center">
				<span className="text-xs font-medium">{title}</span>
			</div>
			<div className="ml-auto flex shrink-0 items-center gap-1">
				{showConnection && <ConnectionIndicator />}
				<button
					type="button"
					className="hover:bg-muted flex cursor-pointer items-center gap-1 rounded-sm px-2 py-1 text-xs transition-colors"
					onClick={() => toggleDark()}
					title={isDark ? "切换到浅色" : "切换到深色"}
					aria-label={isDark ? "切换到浅色" : "切换到深色"}
				>
					<span className="text-muted-foreground text-xs">
						{isDark ? "深色" : "浅色"}
					</span>
					{isDark ? <Moon className="size-4" /> : <Sun className="size-4" />}
				</button>
			</div>
		</div>
	);
}
