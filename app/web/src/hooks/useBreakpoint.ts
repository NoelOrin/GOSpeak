import { createSignal, onCleanup, onMount } from "solid-js";

/** 监听 CSS media query，返回响应式 boolean signal */
export function useMediaQuery(query: string) {
	const getMatch = () =>
		typeof window !== "undefined" ? window.matchMedia(query).matches : false;

	const [matches, setMatches] = createSignal(getMatch());

	onMount(() => {
		const mql = window.matchMedia(query);
		const onChange = (e: MediaQueryListEvent) => setMatches(e.matches);
		setMatches(mql.matches);
		mql.addEventListener("change", onChange);
		onCleanup(() => mql.removeEventListener("change", onChange));
	});

	return matches;
}

/** < md (768px) 视为移动端壳 */
export function useIsMobile() {
	return useMediaQuery("(max-width: 767px)");
}
