import { Toaster } from "@gospeak/components";
import { useThemeStore } from "@/stores/themeStore";

/**
 * 全局 toast 挂载点。
 *
 * 必须在应用根部渲染一次，否则所有 `toast.*` 调用都是空操作。
 * sonner 的 theme 走 prefers-color-scheme，与应用内手动的 `.dark` class 不同步，
 * 所以这里订阅主题 store 显式传入，保证深色模式下 toast 配色一致。
 */
export default function AppToaster() {
	const isDark = useThemeStore((s) => s.isDark);
	return (
		<Toaster
			richColors
			position="top-center"
			theme={isDark ? "dark" : "light"}
			closeButton
			toastOptions={{ duration: 4000 }}
		/>
	);
}
