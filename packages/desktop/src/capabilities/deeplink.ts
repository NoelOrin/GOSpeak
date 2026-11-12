import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

/**
 * Deep link handling, e.g. `gospeak://room/abc123`.
 * In the browser this is a no-op (links open the web app directly).
 */
export function onDeepLink(handler: (url: string) => void): () => void {
	if (!isDesktop()) return () => {};
	return bridge.listen("deeplink.open", (payload) =>
		handler(payload as string),
	);
}

/** Current pending deep link (if the app was launched from one). */
export function getInitialDeepLink(): Promise<string | null> {
	if (!isDesktop()) return Promise.resolve(null);
	return bridge.invoke<string | null>(command("DeepLink", "Initial"));
}
