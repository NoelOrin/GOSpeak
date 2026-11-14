import { debounce, throttle } from "lodash-es";
import { useCallback, useEffect, useMemo, useRef } from "react";
import { socketStore } from "@/stores/socketStore";

// 与服务端 2s 广播节流配合：前端 1.2s 一次心跳，房间里的提示不会闪断
const HEARTBEAT_MS = 1200;
// 停手 3s 视为停止输入，主动发 stop，不必等服务端/前端的 TTL 过期
const IDLE_MS = 3000;

/**
 * 文字房输入框的「正在输入」上报。
 *
 * 只对上报做节流，**不要节流 setContent**，否则会丢字。
 * 返回的 notify 在每次输入时调用，stop 在发送 / 清空 / 失焦时调用。
 * 卸载与切房间会取消在途定时器，避免把旧频道的 stop 发到新频道。
 */
export function useTypingReporter(
	room: string | null,
	domainUUID: string | null,
) {
	const paramsRef = useRef({ room, domainUUID });
	paramsRef.current = { room, domainUUID };

	const emit = useCallback((typing: boolean) => {
		const { room: r, domainUUID: d } = paramsRef.current;
		if (!r) return;
		socketStore.emitTyping(r, d, typing);
	}, []);

	const timers = useMemo(
		() => ({
			heartbeat: throttle(() => emit(true), HEARTBEAT_MS, {
				leading: true,
				trailing: true,
			}),
			idle: debounce(() => emit(false), IDLE_MS),
		}),
		[emit],
	);

	const notify = useCallback(() => {
		if (!paramsRef.current.room) return;
		timers.heartbeat();
		timers.idle();
	}, [timers]);

	const stop = useCallback(() => {
		timers.heartbeat.cancel();
		timers.idle.cancel();
		emit(false);
	}, [timers, emit]);

	useEffect(
		() => () => {
			timers.heartbeat.cancel();
			timers.idle.cancel();
			emit(false);
		},
		[timers, emit],
	);

	// biome-ignore lint/correctness/useExhaustiveDependencies: room/domainUUID 为有意触发器（切房/切域即取消挂起计时器）
	useEffect(() => {
		timers.heartbeat.cancel();
		timers.idle.cancel();
	}, [room, domainUUID, timers]);

	return { notify, stop };
}
