import { Moon, Sun } from "lucide-react";
import { useEffect } from "react";
import { useRouteTitle } from "@/components/titleBar/useRouteTitle";
import { initTheme, toggleDark, useThemeStore } from "@/stores/themeStore";
import ConnectionIndicator from "./connectionIndicator";

export default function Header() {
	const isDark = useThemeStore((s) => s.isDark);
	// 与桌面标题栏共用同一套标题推导
	const title = useRouteTitle();

	useEffect(() => {
		initTheme();
	}, []);

	return (
		<div className="text-foreground flex select-none items-center justify-between overflow-hidden px-2 py-0.5 text-xs">
			<div className="flex-1 font-semibold" />
			<div className="flex items-center justify-center">{title}</div>
			<div className="flex flex-1 items-center justify-end gap-1">
				<ConnectionIndicator />
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
