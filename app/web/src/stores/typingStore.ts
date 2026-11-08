import { create } from "zustand";

/**
 * 文字房「正在输入」状态。
 *
 * 服务端只做转发 + 节流（每 2s 一次心跳），不保证送达，也不发 stop 兜底
 * （浏览器崩溃时收不到 stop）。因此这里每条状态自带过期时间，展示侧按
 * Date.now() 过滤，过期即自动消失，不需要服务端参与清理。
 */
export const TYPING_TTL_MS = 6000;

export interface TypingEventPayload {
	room?: string;
	domain_uuid?: string | null;
	identity?: string;
	typing?: boolean;
}

export function roomTypingKey(
	room: string,
	domainUUID?: string | null,
): string {
	return domainUUID ? `${domainUUID}:${room}` : room;
}

interface TypingState {
	/** roomKey -> identity -> 过期时间戳 */
	byRoom: Record<string, Record<string, number>>;
	apply: (payload: TypingEventPayload) => void;
	clearKey: (key: string) => void;
}

export const useTypingStore = create<TypingState>((set) => ({
	byRoom: {},
	apply: (payload) => {
		const { room, identity } = payload;
		if (!room || !identity) return;
		const key = roomTypingKey(room, payload.domain_uuid);
		set((prev) => {
			const entry = { ...(prev.byRoom[key] ?? {}) };
			if (payload.typing) {
				entry[identity] = Date.now() + TYPING_TTL_MS;
			} else {
				delete entry[identity];
			}
			return { byRoom: { ...prev.byRoom, [key]: entry } };
		});
	},
	clearKey: (key) =>
		set((prev) => {
			if (!prev.byRoom[key]) return prev;
			const next = { ...prev.byRoom };
			delete next[key];
			return { byRoom: next };
		}),
}));

/** 供 socketEvents 的服务端推送处理器调用。 */
export function applyTypingEvent(payload: TypingEventPayload): void {
	useTypingStore.getState().apply(payload);
}

/** 返回某个房间当前仍然有效的、排除自己之后的正在输入者。 */
export function activeTypingIdentities(
	byRoom: Record<string, Record<string, number>>,
	key: string,
	selfIdentity: string,
	now = Date.now(),
): string[] {
	const entry = byRoom[key];
	if (!entry) return [];
	return Object.entries(entry)
		.filter(
			([identity, expiresAt]) => identity !== selfIdentity && expiresAt > now,
		)
		.map(([identity]) => identity);
}
