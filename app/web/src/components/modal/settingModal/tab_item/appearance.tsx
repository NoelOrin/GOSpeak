import {
	useThemeStore,
	darkThemes,
	lightThemes,
	setDarkMode,
	switchTheme,
	type ThemeOption,
} from "@/stores/themeStore";
import { Page, Section, Toggle } from "./shared";
import type { SettingTabConfig } from "./types";

function ThemeCard({
	option,
	active,
	onSelect,
}: {
	option: ThemeOption;
	active: boolean;
	onSelect: () => void;
}) {
	return (
		<button
			type="button"
			className={`cursor-pointer rounded-lg border p-3 text-left transition ${
				active
					? "border-foreground/50 bg-muted/70 ring-foreground/15 ring-1"
					: "border-border hover:border-foreground/25"
			}`}
			onClick={onSelect}
			data-theme={option.name}
		>
			<div className="mb-2 flex gap-1">
				<span className="bg-primary size-3 rounded-full" />
				<span className="bg-secondary size-3 rounded-full" />
				<span className="bg-accent size-3 rounded-full" />
				<span className="bg-neutral size-3 rounded-full" />
			</div>
			<div className="text-foreground text-sm font-medium">{option.label}</div>
			<div className="text-faint-foreground text-2xs">{option.name}</div>
		</button>
	);
}

function AppearanceForm() {
	const isDark = useThemeStore((s) => s.isDark);
	const darkTheme = useThemeStore((s) => s.darkTheme);
	const lightTheme = useThemeStore((s) => s.lightTheme);
	const themes = isDark ? darkThemes : lightThemes;
	const activeTheme = isDark ? darkTheme : lightTheme;

	return (
		<Page
			title="外观"
			desc="浅色与深色主题分开保存。打开深色模式后，下方列表切换为深色主题配置；关闭则配置浅色主题。"
		>
			<Section title="显示模式">
				<Toggle
					label="深色模式"
					desc="关闭时配置浅色主题；开启时切换到深色模式，并配置深色主题。两种选择会分别记住。"
					checked={isDark}
					onChange={setDarkMode}
				/>
			</Section>

			<Section title={isDark ? "深色主题" : "浅色主题"}>
				<p className="text-faint-foreground -mt-1 mb-1 text-xs leading-relaxed">
					当前正在配置
					<span className="text-muted-foreground mx-1 font-medium">
						{isDark ? "深色" : "浅色"}
					</span>
					模式主题。已选：
					<span className="text-muted-foreground mx-1 font-medium">
						{activeTheme}
					</span>
				</p>
				<div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
					{themes.map((option) => (
						<ThemeCard
							key={option.name}
							option={option}
							active={activeTheme === option.name}
							onSelect={() => switchTheme(option.name)}
						/>
					))}
				</div>
			</Section>
		</Page>
	);
}

const appearance: SettingTabConfig = {
	id: "appearance",
	label: "外观",
	icon: "palette",
	component: AppearanceForm,
};

export default appearance;
