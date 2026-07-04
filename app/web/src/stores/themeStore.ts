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

const _darkThemeNames = new Set(darkThemes.map((t) => t.name));

function getInitialDark(): boolean {
	const saved = localStorage.getItem(STORAGE_DARK);
	if (saved !== null) return saved === "true";
	return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

function getInitialTheme(isDark: boolean): string {
	if (isDark) {
		return localStorage.getItem(STORAGE_THEME_DARK) || "dark";
	}
	return localStorage.getItem(STORAGE_THEME_LIGHT) || "acid";
}

const initialDark = getInitialDark();
const initialTheme = getInitialTheme(initialDark);

const [store, setStore] = createStore({
	theme: initialTheme,
	isDark: initialDark,
});

function applyTheme() {
	document.documentElement.setAttribute("data-theme", store.theme);
	document.documentElement.classList.toggle("dark", store.isDark);
}

function switchTheme(name: string) {
	setStore("theme", name);
	const storageKey = store.isDark ? STORAGE_THEME_DARK : STORAGE_THEME_LIGHT;
	localStorage.setItem(storageKey, name);
	applyTheme();
	console.log("[Theme] theme:", name, "| isDark:", store.isDark);
}

function toggleDark() {
	const next = !store.isDark;
	setStore("isDark", next);
	localStorage.setItem(STORAGE_DARK, String(next));
	// 切换到该模式下上次选择的主题
	const theme = next
		? localStorage.getItem(STORAGE_THEME_DARK) || "dark"
		: localStorage.getItem(STORAGE_THEME_LIGHT) || "acid";
	setStore("theme", theme);
	applyTheme();
	console.log("[Theme] theme:", theme, "| isDark:", next);
}

function initTheme() {
	applyTheme();
}

export { initTheme, switchTheme, toggleDark };
export default store;
