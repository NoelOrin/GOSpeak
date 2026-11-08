/**
 * Native bridge between the embedded web frontend and the GOSpeak desktop host.
 *
 * Built on the official `mygo-runtime` package: MyGo injects its runtime into
 * every page, and this module wraps it for the capability SDK:
 *   - `call(method, ...args)` invokes a Go method bound with `mygo.Bind`
 *     (method names are "<Struct>.<Method>", e.g. `Overlay.PushState`), with
 *     POSITIONAL JSON-decoded arguments;
 *   - `on(event, listener)` receives events the host emits with
 *     `mygo.NewEvent[T](name)` + `.Broadcast/.Emit`.
 *
 * Outside a MyGo window (a plain browser tab of the dev server) `isMyGo()` is
 * false, `call` rejects and listeners never fire, so the same code path works
 * in both environments — see `src/capabilities/*` for the browser fallbacks.
 */
import * as mygo from "mygo-runtime";

export interface DesktopBridge {
	/** Invoke a bound Go method with positional args. */
	invoke<T = unknown>(command: string, ...args: unknown[]): Promise<T>;
	/** Subscribe to a host-emitted event. Returns an unsubscribe function. */
	listen(event: string, handler: (...args: unknown[]) => void): () => void;
}

/**
 * Builds the command key MyGo registers for a bound service method.
 *
 * MyGo binds `mygo.Bind(Window{})` as `"Window"` and each exported method as
 * `"<Struct>.<Method>"` — `"Window.Minimize"`, `"ServerConfig.GetBaseURL"`.
 * Call this instead of writing the string by hand: a mismatch rejects at
 * runtime with "method X is not bound" and is easy to miss, because most
 * capability methods swallow their errors. `scripts/check-desktop-commands.mjs`
 * verifies every call site against the Go side in CI.
 */
export function command(service: string, method: string): string {
	return `${service}.${method}`;
}

/**
 * Singleton transport used by every capability module. In the browser it
 * degrades gracefully (rejects `invoke`, no-ops `listen`) so the same component
 * code works in both the desktop client and a plain browser.
 */
export const bridge: DesktopBridge = {
	invoke<T = unknown>(command: string, ...args: unknown[]): Promise<T> {
		if (!mygo.isMyGo()) {
			return Promise.reject(
				new Error(
					`[gospeak-desktop] "${command}" requires the desktop runtime; ` +
						`running in browser fallback.`,
				),
			);
		}
		return mygo.call<T>(command, ...args);
	},
	listen(event: string, handler: (...args: unknown[]) => void): () => void {
		if (!mygo.isMyGo()) return () => {};
		return mygo.on<unknown>(event, handler);
	},
};
