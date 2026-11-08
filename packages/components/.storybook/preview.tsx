import type { Preview } from "@storybook/react-vite";
import "./preview.css";

const preview: Preview = {
	parameters: { layout: "centered" },
	globalTypes: {
		theme: {
			defaultValue: "light",
			toolbar: { items: ["light", "dark"], title: "Theme" },
		},
	},
	decorators: [
		// Radix Portal 渲染在 body 下，主题必须落在 <html> 上才能覆盖所有层
		(Story, context) => {
			const isDark = context.globals.theme === "dark";
			const html = document.documentElement;
			html.setAttribute("data-theme", isDark ? "dark" : "light");
			html.classList.toggle("dark", isDark);
			document.body.style.background = isDark
				? "var(--background)"
				: "var(--muted)";
			return (
				<div className="bg-background text-foreground rounded p-4">
					<Story />
				</div>
			);
		},
	],
};
export default preview;
