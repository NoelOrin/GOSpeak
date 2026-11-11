/**
 * @gospeak/desktop
 *
 * Desktop-platform capability SDK for GOSpeak. This is the TypeScript side of
 * the bridge between the embedded web frontend (`@gospeak/web`) and the native
 * desktop host (`app/desktop`, a Go application built on the MyGo framework,
 * `github.com/egoist/mygo`).
 *
 * Every capability exposes a native implementation (routed to Go via the
 * official `mygo-runtime` package — `call("<struct>.<method>", ...args)` and
 * `on(event, listener)`) and a safe browser fallback, so the same component
 * code works both inside the desktop client and in a plain browser.
 */

export { bridge } from "./bridge";
export type { DesktopBridge } from "./bridge";

export { isDesktop, runtime } from "./runtime";
export type { Runtime } from "./runtime";

export {
	BUILD_TARGET,
	DESKTOP_BUILD,
	WEB_BUILD,
	inDesktop,
	assertDesktop,
	ifDesktop,
	desktopOnly,
	webOnly,
	importDesktop,
	importWeb,
} from "./conditional";
export type { BuildTarget } from "./conditional";

export {
	createWindowControls,
	type WindowControls,
} from "./capabilities/window";

export {
	createSystemTray,
	type SystemTray,
	type TrayMenuItem,
} from "./capabilities/tray";

export {
	showNotification,
	requestNotificationPermission,
	type NotificationOptions,
	type NotificationHandle,
} from "./capabilities/notification";

export { createAutoStart, type AutoStart } from "./capabilities/autostart";

export {
	createOverlay,
	type Overlay,
	type OverlayMember,
	type OverlaySnapshot,
	type OverlayPolicy,
	type OverlayConfig,
} from "./capabilities/overlay";

export { onDeepLink, getInitialDeepLink } from "./capabilities/deeplink";

export {
	openFileDialog,
	saveFileDialog,
	showMessageBox,
	type OpenDialogOptions,
	type SaveDialogOptions,
	type MessageBoxOptions,
} from "./capabilities/dialog";

export {
	createUpdater,
	type Updater,
	type UpdateInfo,
} from "./capabilities/updater";

export {
	createServerConfig,
	SERVER_BASE_URL_KEY,
	type ServerConfig,
} from "./capabilities/server";

import { createWindowControls } from "./capabilities/window";
import { createSystemTray } from "./capabilities/tray";
import { createAutoStart } from "./capabilities/autostart";
import { createUpdater } from "./capabilities/updater";
import { createOverlay } from "./capabilities/overlay";
import { isDesktop } from "./runtime";

/**
 * Single object that bundles every desktop capability. Use this from the web
 * app when you want one import instead of individual `create*` factories.
 *
 * In the browser all members are inert fallbacks; in the desktop host they map
 * to native implementations.
 */
export function createDesktop() {
	return {
		isDesktop: isDesktop(),
		window: createWindowControls(),
		tray: createSystemTray(),
		autostart: createAutoStart(),
		updater: createUpdater(),
		overlay: createOverlay(),
	};
}

export type Desktop = ReturnType<typeof createDesktop>;
