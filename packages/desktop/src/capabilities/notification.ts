import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

export interface NotificationOptions {
	title: string;
	body?: string;
	/** Sound / icon hint understood by the host. */
	icon?: string;
}

export interface NotificationHandle {
	/** Clicking the notification (desktop only). */
	onClick(handler: () => void): void;
}

/** Native desktop notifications. Falls back to the Web Notifications API in browser. */
export function showNotification(
	options: NotificationOptions,
): NotificationHandle {
	if (isDesktop()) {
		const handlers = new Set<() => void>();
		bridge
			.invoke<string>(command("Notification", "Show"), options)
			.then((eventId) => {
				// Host broadcasts "notification.click" with the ID as payload;
				// match it here so every notification shares one event channel.
				bridge.listen("notification.click", (id) => {
					if (id !== eventId) return;
					for (const handler of handlers) handler();
				});
			})
			.catch(() => {});
		return {
			onClick(handler) {
				handlers.add(handler);
			},
		};
	}

	// Browser fallback via the Web Notifications API.
	const n = new Notification(options.title, {
		body: options.body,
		icon: options.icon,
	});
	const handlers = new Set<() => void>();
	n.onclick = () => {
		for (const handler of handlers) handler();
	};
	return {
		onClick(handler) {
			handlers.add(handler);
		},
	};
}

/** Request/check notification permission (mirrors the browser API). */
export function requestNotificationPermission(): Promise<NotificationPermission> {
	if (isDesktop()) {
		return bridge.invoke<NotificationPermission>(
			command("Notification", "RequestPermission"),
		);
	}
	if (typeof Notification === "undefined") return Promise.resolve("denied");
	return Notification.requestPermission();
}
