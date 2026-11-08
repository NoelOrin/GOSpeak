import { isMyGo } from "mygo-runtime";

/**
 * True when the code is running inside the GOSpeak desktop host, i.e. MyGo has
 * injected its runtime and we are in a real window rather than a browser tab of
 * the dev server.
 */
export function isDesktop(): boolean {
	return isMyGo();
}

/** The active runtime. Useful for conditional UI ("only show in desktop"). */
export type Runtime = "web" | "desktop";

export const runtime: Runtime = isDesktop() ? "desktop" : "web";
