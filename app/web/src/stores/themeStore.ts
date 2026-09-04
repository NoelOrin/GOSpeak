import { createStore } from "solid-js/store";

const STORAGE_THEME_LIGHT = "theme-light";
const STORAGE_THEME_DARK = "theme-dark";
const STORAGE_DARK = "theme-is-dark";

export interface ThemeOption {
	name: string;
	label: string;
}

export const lightThemes: ThemeOption[] = [
	{ name: "acid", label: "Acid" },
	{ name: "aqua", label: "Aqua" },
	{ name: "autumn", label: "Autumn" },
	{ name: "bumblebee", label: "Bumblebee" },
	{ name: "caramellatte", label: "Caramel" },
	{ name: "cupcake", label: "Cupcake" },
	{ name: "emerald", label: "Emerald" },
	{ name: "fantasy", label: "Fantasy" },
	{ name: "garden", label: "Garden" },
	{ name: "lemonade", label: "Lemonade" },
	{ name: "light", label: "Light" },
	{ name: "lofi", label: "Lo-Fi" },
	{ name: "nord", label: "Nord" },
	{ name: "pastel", label: "Pastel" },
	{ name: "retro", label: "Retro" },
	{ name: "valentine", label: "Valentine" },
	{ name: "winter", label: "Winter" },
	{ name: "wireframe", label: "Wireframe" },
];

export const darkThemes: ThemeOption[] = [
	{ name: "abyss", label: "Abyss" },
	{ name: "black", label: "Black" },
	{ name: "business", label: "Business" },
	{ name: "coffee", label: "Coffee" },
	{ name: "cmyk", label: "CMYK" },
	{ name: "cyberpunk", label: "Cyberpunk" },
	{ name: "dark", label: "Dark" },
	{ name: "dim", label: "Dim" },
	{ name: "dracula", label: "Dracula" },
	{ name: "forest", label: "Forest" },
	{ name: "halloween", label: "Halloween" },
	{ name: "luxury", label: "Luxury" },
	{ name: "night", label: "Night" },
	{ name: "palenight", label: "Palenight" },
	{ name: "sunset", label: "Sunset" },
	{ name: "synthwave", label: "Synthwave" },
];

function getInitialDark(): boolean {
	const saved = localStorage.getItem(STORAGE_DARK);
	if (saved !== null) return saved === "true";
	return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function getStoredTheme(key: string, fallback: string): string {
	return localStorage.getItem(key) || fallback;
}

const initialDark = getInitialDark();
const initialLightTheme = getStoredTheme(STORAGE_THEME_LIGHT, "acid");
const initialDarkTheme = getStoredTheme(STORAGE_THEME_DARK, "dark");
const initialTheme = initialDark ? initialDarkTheme : initialLightTheme;

const [store, setStore] = createStore({
	theme: initialTheme,
	lightTheme: initialLightTheme,
	darkTheme: initialDarkTheme,
	isDark: initialDark,
});

function applyTheme() {
	document.documentElement.setAttribute("data-theme", store.theme);
	document.documentElement.classList.toggle("dark", store.isDark);
}

function switchLightTheme(name: string) {
	setStore("lightTheme", name);
	localStorage.setItem(STORAGE_THEME_LIGHT, name);
	if (!store.isDark) {
		setStore("theme", name);
		applyTheme();
	}
}

function switchDarkTheme(name: string) {
	setStore("darkTheme", name);
	localStorage.setItem(STORAGE_THEME_DARK, name);
	if (store.isDark) {
		setStore("theme", name);
		applyTheme();
	}
}

/** 兼容旧调用：按当前模式写入对应主题 */
function switchTheme(name: string) {
	if (store.isDark) {
		switchDarkTheme(name);
		return;
	}
	switchLightTheme(name);
}

function toggleDark() {
	const next = !store.isDark;
	const theme = next ? store.darkTheme : store.lightTheme;
	setStore({
		isDark: next,
		theme,
	});
	localStorage.setItem(STORAGE_DARK, String(next));
	applyTheme();
}

function setDarkMode(enabled: boolean) {
	if (enabled === store.isDark) return;
	toggleDark();
}

function initTheme() {
	applyTheme();
}

export {
	initTheme,
	setDarkMode,
	switchDarkTheme,
	switchLightTheme,
	switchTheme,
	toggleDark,
};
export default store;
