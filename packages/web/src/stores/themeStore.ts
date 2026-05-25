import { createStore } from "solid-js/store";

export const ThemeEnum: Record<string, string> = {
	light: "acid",
	dark: "synthwave",
};

const [themeStore, setThemeStore] = createStore({
	themeVar: localStorage.getItem("theme") || "light",
	theme: ThemeEnum[localStorage.getItem("theme") || "light"],
});

const switchTheme = () => {
	const nextTheme = themeStore.themeVar === "light" ? "dark" : "light";
	console.log(nextTheme);

	setThemeStore({ themeVar: nextTheme });
	localStorage.setItem("theme", nextTheme);

	document.documentElement.classList.toggle("dark", nextTheme === "dark");
	document.documentElement.setAttribute("data-theme", ThemeEnum[nextTheme]);
};

const setThemeVar = (theme: string) => {
	setThemeStore({ themeVar: theme });
};

const initTheme = () => {
	document.documentElement.classList.toggle(
		"dark",
		themeStore.themeVar === "dark",
	);
	document.documentElement.setAttribute(
		"data-theme",
		ThemeEnum[themeStore.themeVar],
	);
};

const ThemeStore = {
	themeVar: themeStore.themeVar,
	theme: themeStore.theme,
	setTheme: setThemeVar,
	switchTheme,
	initTheme,
};

export default ThemeStore;
