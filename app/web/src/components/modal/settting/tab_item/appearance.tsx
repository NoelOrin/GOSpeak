import { For } from "solid-js";
import themeStore, {
	darkThemes,
	lightThemes,
	setDarkMode,
	switchTheme,
	type ThemeOption,
} from "@/stores/themeStore";
import { Page, Section, Toggle } from "./shared";
import type { SettingTabConfig } from "./types";

const ThemeCard = (props: {
	option: ThemeOption;
	active: boolean;
	onSelect: () => void;
}) => (
	<button
		type="button"
		class="cursor-pointer rounded-box border p-3 text-left transition"
		classList={{
			"border-base-content/50 bg-base-200/70 ring-1 ring-base-content/15":
				props.active,
			"border-base-300 hover:border-base-content/25": !props.active,
		}}
		onClick={props.onSelect}
		data-theme={props.option.name}
	>
		<div class="mb-2 flex gap-1">
			<span class="h-3 w-3 rounded-full bg-primary" />
			<span class="h-3 w-3 rounded-full bg-secondary" />
			<span class="h-3 w-3 rounded-full bg-accent" />
			<span class="h-3 w-3 rounded-full bg-neutral" />
		</div>
		<div class="text-sm font-medium text-base-content">
			{props.option.label}
		</div>
		<div class="text-[11px] text-base-content/50">{props.option.name}</div>
	</button>
);

const AppearanceForm = () => {
	const themes = () => (themeStore.isDark ? darkThemes : lightThemes);
	const activeTheme = () =>
		themeStore.isDark ? themeStore.darkTheme : themeStore.lightTheme;

	return (
		<Page
			title="外观"
			desc="浅色与深色主题分开保存。打开深色模式后，下方列表切换为深色主题配置；关闭则配置浅色主题。"
		>
			<Section title="显示模式">
				<Toggle
					label="深色模式"
					desc="关闭时配置浅色主题；开启时切换到深色模式，并配置深色主题。两种选择会分别记住。"
					checked={themeStore.isDark}
					onChange={setDarkMode}
				/>
			</Section>

			<Section title={themeStore.isDark ? "深色主题" : "浅色主题"}>
				<p class="-mt-1 mb-1 text-xs leading-relaxed text-base-content/50">
					当前正在配置
					<span class="mx-1 font-medium text-base-content/70">
						{themeStore.isDark ? "深色" : "浅色"}
					</span>
					模式主题。已选：
					<span class="mx-1 font-medium text-base-content/70">
						{activeTheme()}
					</span>
				</p>
				<div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
					<For each={themes()}>
						{(option) => (
							<ThemeCard
								option={option}
								active={activeTheme() === option.name}
								onSelect={() => switchTheme(option.name)}
							/>
						)}
					</For>
				</div>
			</Section>
		</Page>
	);
};

const appearance: SettingTabConfig = {
	id: "appearance",
	label: "外观",
	icon: "palette",
	component: AppearanceForm,
};

export default appearance;
