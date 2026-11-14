/**
 * LOCKED JOIN ORCHESTRATOR
 *
 * Do NOT modify this file for SFU provider adaptation (LiveKit).
 * Provider-specific connect targets, token fields, etc. belong in:
 * - app/web/src/components/room/session/providers.ts
 * - app/web/src/components/room/session/runVoiceJoin.ts
 * - packages/sfu-client/*
 * - app/web/src/api/sfu.ts
 *
 * This hook only owns: token query, session phase, abort/teardown lifecycle, UI state.
 * SFU adaptation changes that touch this file are rejected.
 *
 * React 版：与 Solid 版语义 1:1 对应——
 * - createSignal → useSessionReducer/useRef
 * - createEffect(on(tokenQuery.data)) → token 数据到达时的 useEffect
 * - createEffect(session 解构) → session/selectedRoom 变化的 useEffect
 */

import type { SFUClient } from "@gospeak/sfu-client/types";
import { useQuery } from "@tanstack/react-query";
import { toast } from "@gospeak/components";
import { useCallback, useEffect, useRef, useState } from "react";
import { getJoinToken, resolveSFUProvider } from "@/api/sfu";
import { cleanupAudioHandler, setupAudioHandler } from "@/handler_audio";
import AudioDeviceStore from "@/stores/audioDeviceStore";
import { socketStore, useSocketStore } from "@/stores/socketStore";
import { useUserStore } from "@/stores/userStore";
import { loadSfuClient } from "../services/loadSfuClient";
import { getVoiceProviderAdapter } from "../session/providers";
import { runVoiceJoin, VoiceJoinAbortError } from "../session/runVoiceJoin";
import {
	type Session,
	clearReconnectingToast,
	showReconnectingToast,
} from "../session/voiceSessionUi";
import {
	isVoiceInteractive,
	isVoiceLoading,
	type VoicePhase,
	voicePhaseLabel,
} from "../session/voiceSessionTypes";

export function useVoiceSession() {
	const selectedRoom = useSocketStore((s) => s.selectedRoomInfo);
	const [session, setSession] = useState<Session | null>(null);

	// refs 保存可变编排状态（与 Solid 版闭包变量一一对应）
	const joinAbortControllerRef = useRef<AbortController | null>(null);
	const activeJoinClientRef = useRef<SFUClient | null>(null);
	const leavingRef = useRef(new WeakSet<SFUClient>());
	const reconnectingClientsRef = useRef(new WeakSet<SFUClient>());
	// session 的镜像 ref：回调里读取最新值，避免闭包过期
	const sessionRef = useRef<Session | null>(null);
	sessionRef.current = session;

	const abortCurrentJoin = useCallback(() => {
		joinAbortControllerRef.current?.abort();
		joinAbortControllerRef.current = null;
	}, []);

	const tokenQuery = useQuery({
		queryKey: ["token", selectedRoom?.domain_uuid, selectedRoom?.name],
		enabled: !!selectedRoom,
		queryFn: async ({ signal }) => {
			const room = useSocketStore.getState().selectedRoomInfo;
			if (!room) {
				throw new Error("room is missing");
			}
			return getJoinToken(
				{
					room: room.name,
					identity: useUserStore.getState().user?.name ?? "",
					password: (room as { _password?: string })._password,
					domain_uuid: room.domain_uuid,
				},
				signal,
			);
		},
		retry: false,
		staleTime: 5 * 60 * 1000,
	});

	// 信令重连后的自愈（会话生命周期，不属于 provider 适配）：
	// 断网时 LiveKit 可能和信令一起掉，此时 session 为 null 或 failed，
	// 重连成功后重新取一次 token 触发完整 re-join。
	// 媒体还活着的情况不在这里处理——那由 socketStore 把信令面的房间注册补回来即可。
	const connected = useSocketStore((s) => s.connected);
	const wasConnectedRef = useRef(connected);
	useEffect(() => {
		const was = wasConnectedRef.current;
		wasConnectedRef.current = connected;
		if (was || !connected || !selectedRoom) return;
		const status = sessionRef.current?.status;
		if (!sessionRef.current || status === "failed") {
			void tokenQuery.refetch();
		}
	}, [connected, selectedRoom, tokenQuery]);

	const phase: VoicePhase = session?.status ?? "idle"; // VoiceChat 加载条件：
	// - 通用：phase interactive（ready/media_ready/reconnecting）
	// - 媒体先就绪：client 挂上即加载（interactiveAfterMedia），不堵信令
	const isJoined = (() => {
		const s = session;
		if (
			s?.client &&
			s.status !== "failed" &&
			s.status !== "leaving" &&
			s.provider &&
			getVoiceProviderAdapter(s.provider).interactiveAfterMedia
		) {
			return true;
		}
		return isVoiceInteractive(phase);
	})();
	const isReconnecting = phase === "reconnecting";
	const isLoading = !isJoined && isVoiceLoading(phase);
	const phaseLabel = voicePhaseLabel(phase);
	const currentRoom = session?.roomName ?? null;
	const sfuClient = session?.client ?? null;
	const error = session?.error ?? null;

	const teardownClient = useCallback(async (client: SFUClient) => {
		if (leavingRef.current.has(client)) return;
		leavingRef.current.add(client);
		reconnectingClientsRef.current.delete(client);
		await client.leaveRoom().catch(() => {});
		await client.destroy().catch(() => {});
		if (activeJoinClientRef.current === client) {
			activeJoinClientRef.current = null;
		}
	}, []);

	const teardownSession = useCallback(
		async (sess: Session) => {
			clearReconnectingToast();
			setSession((cur) =>
				cur === sess ? { ...cur, status: "leaving", error: null } : cur,
			);
			if (sess.client) {
				await teardownClient(sess.client);
			} else if (activeJoinClientRef.current) {
				await teardownClient(activeJoinClientRef.current);
			}
			cleanupAudioHandler();
			await socketStore
				.leaveRoom(sess.roomName, sess.domain_uuid)
				.catch(() => {});
			setSession((cur) => (cur === sess ? null : cur));
			socketStore.clearCurrentRoom();
		},
		[teardownClient],
	);

	// selectedRoom 清空：abort + teardown（对应 Solid effect 1）
	useEffect(() => {
		if (!selectedRoom) {
			abortCurrentJoin();
			const toTeardown =
				sessionRef.current?.client ?? activeJoinClientRef.current;
			if (toTeardown) void teardownClient(toTeardown);
			cleanupAudioHandler();
			setSession(null);
			socketStore.clearCurrentRoom();
			socketStore.setCurrentSFUProvider(undefined);
		}
	}, [selectedRoom, abortCurrentJoin, teardownClient]);

	// 域切换时清理跨域会话（对应 Solid effect 2）
	useEffect(() => {
		const domain = socketStore.currentDomainUUID();
		const sess = sessionRef.current;
		if (sess?.domain_uuid && sess.domain_uuid !== (domain || undefined)) {
			void socketStore
				.leaveRoom(sess.roomName, sess.domain_uuid)
				.catch(() => {});
		}
		if (selectedRoom && (selectedRoom.domain_uuid || "") !== (domain || "")) {
			socketStore.clearSelectedRoom();
		}
	}, [selectedRoom]);

	// token 到达 → 发起 join（对应 Solid effect on(tokenQuery.data)）
	// 依赖数组已完整覆盖 tokenQuery.data / abortCurrentJoin / teardownClient / teardownSession，
	// 无需 lint suppression（此处原有的失效 eslint 注释已删除）。
	useEffect(() => {
		const data = tokenQuery.data;
		if (!data) return;

		abortCurrentJoin();

		const controller = new AbortController();
		joinAbortControllerRef.current = controller;
		const { signal } = controller;

		const provider = resolveSFUProvider(data);
		socketStore.setCurrentSFUProvider(provider);
		const roomPassword = (
			useSocketStore.getState().selectedRoomInfo as {
				_password?: string;
			} | null
		)?._password;
		const audioSettings = AudioDeviceStore.state;
		const newRoom = data.room;
		const prev = sessionRef.current;
		const prevClient = activeJoinClientRef.current;

		// 切房：立即退旧业务房（信令面马上发，不阻塞新 join）。
		// SFU client 拆除仍需串行 await，避免与新房 createOffer 竞态。
		const newDomain =
			useSocketStore.getState().selectedRoomInfo?.domain_uuid ??
			socketStore.currentDomainUUID() ??
			undefined;
		const roomChanged =
			!!prev &&
			(prev.roomName !== newRoom ||
				(prev.domain_uuid || "") !== (newDomain || ""));
		if (roomChanged) {
			void socketStore
				.leaveRoom(prev.roomName, prev.domain_uuid)
				.catch(() => {});
		}

		// 立即把 session 切到新房 resolving，UI 显示新房
		setSession({
			roomName: newRoom,
			domain_uuid: newDomain,
			client: null,
			signal,
			status: "resolving",
			provider,
			error: null,
		});

		void (async () => {
			let createdClient: SFUClient | null = null;
			try {
				// 串行拆旧 SFU client（保证 createOffer 不竞态）。
				// 同房重触发时必须先退旧 client，避免媒体会话竞态。
				if (prevClient) {
					await teardownClient(prevClient);
					cleanupAudioHandler();
					activeJoinClientRef.current = null;
				}
				if (signal.aborted) return;

				const { client, provider: joinedProvider } = await runVoiceJoin(data, {
					loadClient: loadSfuClient,
					setupAudio: setupAudioHandler,
					joinSignalRoom: (room, identity, password, domain_uuid) =>
						socketStore.joinRoom(room, identity, password, domain_uuid),
					connectSignal: (workerUrl) => socketStore.connectToWorker(workerUrl),
					getCurrentRoom: () => {
						const room = socketStore.currentRoom();
						const domain = socketStore.currentDomainUUID();
						if (!room) return null;
						const meta = socketStore
							.rooms()
							.find(
								(r) =>
									r.name === room && (r.domain_uuid || "") === (domain || ""),
							);
						return {
							room,
							domain_uuid: domain ?? undefined,
							type: meta?.type,
						};
					},
					rejoinTextRoom: (room, domain_uuid, identity) =>
						socketStore
							.joinRoom(room, identity, undefined, domain_uuid)
							.then(() => undefined),
					joinSignalSfu: (room, identity, stream, domain_uuid) =>
						socketStore.joinRoomSFU(room, identity, stream, domain_uuid),
					onPhase: (nextPhase) => {
						if (signal.aborted) return;
						setSession((s) =>
							s && s.roomName === newRoom && s.signal === signal
								? { ...s, status: nextPhase, error: null }
								: s,
						);
					},
					// 通用 media 完成回调：挂 client 到 session。
					// media_ready 时 VoiceChat/audio bridge 需要 client 已就绪。
					onClientReady: (client, joinedProvider) => {
						if (signal.aborted) return;
						createdClient = client;
						activeJoinClientRef.current = client;
						socketStore.setCurrentSFUProvider(joinedProvider);
						const whipReady =
							!!getVoiceProviderAdapter(joinedProvider).interactiveAfterMedia;
						// interactiveAfterMedia：client 挂上即 media_ready，VoiceChat 可加载。
						// 否则：只挂 client，phase 仍由 onPhase 推进。
						setSession((s) =>
							s && s.roomName === newRoom && s.signal === signal
								? {
										...s,
										client,
										provider: joinedProvider,
										...(whipReady ? { status: "media_ready" as const } : {}),
										error: null,
									}
								: s,
						);
					},
					audioOptions: {
						audioCapture: {
							deviceId: audioSettings.selectedAudioInput || undefined,
							echoCancellation: audioSettings.echoCancellation,
							noiseSuppression: audioSettings.noiseSuppression,
							autoGainControl: audioSettings.autoGainControl,
							voiceIsolation: audioSettings.voiceIsolation,
							sampleRate: audioSettings.sampleRate,
							sampleSize: audioSettings.sampleSize,
							channelCount: audioSettings.stereo ? 2 : 1,
						},
						publishAudio: {
							maxBitrate: audioSettings.audioBitrate,
							dtx: audioSettings.dtx,
							red: audioSettings.red,
							forceStereo: audioSettings.stereo,
						},
					},
					socket: socketStore.getSocket(),
					password: roomPassword,
					signal,
				});
				if (signal.aborted) {
					void teardownClient(client);
					return;
				}

				createdClient = client;
				const joinedClient = client;
				activeJoinClientRef.current = joinedClient;
				socketStore.setCurrentSFUProvider(joinedProvider);

				// join 全部成功后才挂断连/重连回调，避免连接阶段瞬态误报
				joinedClient.onDisconnected((info) => {
					if (signal.aborted) return;
					const cur = sessionRef.current;
					// 仅当当前 session 仍指向该 client 且处于活跃态才视为异常断连
					if (cur?.client !== joinedClient) return;
					if (
						cur?.status !== "ready" &&
						cur?.status !== "media_ready" &&
						cur?.status !== "reconnecting" &&
						!reconnectingClientsRef.current.has(joinedClient)
					) {
						return;
					}
					console.warn("[RoomDetail] SFU disconnected unexpectedly", info);
					const disconnectedMsg =
						info?.reason === "DUPLICATE_IDENTITY"
							? "该账号已在其他设备加入此房间"
							: "连接已断开";
					reconnectingClientsRef.current.delete(joinedClient);
					setSession((s) =>
						s && s.client === joinedClient
							? { ...s, status: "failed", error: disconnectedMsg }
							: s,
					);
					// phase effect 会清 reconnecting toast
					toast.error(disconnectedMsg);
					if (cur) void teardownSession(cur);
				});
				joinedClient.onReconnecting?.(() => {
					if (signal.aborted) return;
					const cur = sessionRef.current;
					if (cur?.client !== joinedClient) return;
					if (
						cur?.status !== "ready" &&
						cur?.status !== "media_ready" &&
						cur?.status !== "reconnecting"
					) {
						return;
					}
					reconnectingClientsRef.current.add(joinedClient);
					setSession((s) =>
						s && s.client === joinedClient
							? { ...s, status: "reconnecting", error: null }
							: s,
					);
					// toast 由 phase effect 统一展示
				});
				joinedClient.onReconnected?.(() => {
					if (signal.aborted) return;
					const cur = sessionRef.current;
					if (cur?.client !== joinedClient) return;
					// 用 WeakSet 标记，规避 session status 尚未 flush 时的漏清
					const wasReconnecting =
						reconnectingClientsRef.current.has(joinedClient) ||
						cur?.status === "reconnecting";
					reconnectingClientsRef.current.delete(joinedClient);
					if (!wasReconnecting) {
						clearReconnectingToast();
						return;
					}
					setSession((s) =>
						s && s.client === joinedClient
							? { ...s, status: "ready", error: null }
							: s,
					);
					clearReconnectingToast();
					toast.success("已重连");
				});

				setSession({
					roomName: newRoom,
					domain_uuid: newDomain,
					client: createdClient,
					signal,
					status: "ready",
					provider: joinedProvider,
					error: null,
				});
			} catch (err) {
				if (
					signal.aborted ||
					err instanceof VoiceJoinAbortError ||
					(err instanceof Error && err.name === "AbortError")
				) {
					if (createdClient) {
						void teardownClient(createdClient);
						activeJoinClientRef.current = null;
					}
					return;
				}
				console.error("[RoomDetail] createSFUClient / joinRoom failed:", err);
				const message =
					err instanceof Error && err.message ? err.message : "加入房间失败";
				setSession((s) =>
					s && s.roomName === newRoom
						? { ...s, status: "failed", error: message, client: null }
						: s,
				);
				toast.error(
					message === "加入房间失败" ? message : `加入房间失败：${message}`,
				);
				if (createdClient) {
					void teardownClient(createdClient);
					cleanupAudioHandler();
					if (activeJoinClientRef.current === createdClient) {
						activeJoinClientRef.current = null;
					}
				}
			}
		})();
	}, [tokenQuery.data, abortCurrentJoin, teardownClient, teardownSession]);

	// token 查询失败（对应 Solid effect on tokenQuery.isError）
	useEffect(() => {
		if (!tokenQuery.isError) return;
		const tokenError = tokenQuery.error as {
			response?: { data?: { msg?: string } };
			message?: string;
		} | null;
		const msg = tokenError?.response?.data?.msg || tokenError?.message || "";
		if (msg.includes("room is full")) {
			toast.error("房间已满，无法加入");
			socketStore.clearSelectedRoom();
			return;
		}
		if (msg) {
			toast.error(`加入房间失败：${msg}`);
			setSession((s) =>
				s
					? { ...s, status: "failed", error: msg }
					: {
							roomName: useSocketStore.getState().selectedRoomInfo?.name ?? "",
							domain_uuid:
								useSocketStore.getState().selectedRoomInfo?.domain_uuid ??
								socketStore.currentDomainUUID() ??
								undefined,
							client: null,
							signal: new AbortController().signal,
							status: "failed",
							error: msg,
						},
			);
		}
	}, [tokenQuery.isError, tokenQuery.error]);

	// toast 生命周期以 phase 为准：离开 reconnecting 必关，避免回调漏清
	useEffect(() => {
		if (isReconnecting) {
			showReconnectingToast();
		} else {
			clearReconnectingToast();
		}
	}, [isReconnecting]);

	const handleManualLeave = useCallback(async () => {
		abortCurrentJoin();
		const sess = sessionRef.current;
		if (sess) await teardownSession(sess);
		else {
			if (activeJoinClientRef.current) {
				await teardownClient(activeJoinClientRef.current);
			}
			cleanupAudioHandler();
		}
		// 兜底：teardownSession 失败时 currentRoom 仍可能残留
		socketStore.clearCurrentRoom();
		socketStore.setCurrentSFUProvider(undefined);
		socketStore.clearSelectedRoom();
	}, [abortCurrentJoin, teardownClient, teardownSession]);

	// 被踢时复用手动离开逻辑：abort 进行中 join + teardown 媒体 + 清状态
	useEffect(() => {
		const dispose = socketStore.onRoomKicked(() => {
			void handleManualLeave();
		});
		return dispose;
	}, [handleManualLeave]);

	return {
		selectedRoom,
		phase,
		sfuClient,
		isJoined,
		isReconnecting,
		isLoading,
		phaseLabel,
		currentRoom,
		error,
		handleManualLeave,
		retry: () => void tokenQuery.refetch(),
		teardownSession,
	};
}
