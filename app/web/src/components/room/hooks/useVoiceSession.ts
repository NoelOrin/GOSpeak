/**
 * LOCKED JOIN ORCHESTRATOR
 *
 * Do NOT modify this file for SFU provider adaptation (LiveKit / SRS / Agora / Cloudflare).
 * Provider-specific connect targets, subscribePeers, WHIP/WHEP, token fields, etc. belong in:
 * - app/web/src/components/room/session/providers.ts
 * - app/web/src/components/room/session/runVoiceJoin.ts
 * - packages/sfu-client/*
 * - app/web/src/api/sfu.ts
 *
 * This hook only owns: token query, session phase, abort/teardown lifecycle, UI state.
 * SFU adaptation changes that touch this file are rejected.
 */

import type { SFUClient, SFUProvider } from "@gospeak/sfu-client/types";
import { useQuery } from "@tanstack/solid-query";
import {
	createEffect,
	createMemo,
	createSignal,
	on,
	onCleanup,
	onMount,
} from "solid-js";
import {
	dismissToast,
	removeToast,
	showToast,
	updateToast,
} from "solid-notifications";
import { getJoinToken, resolveSFUProvider } from "@/api/sfu";
import { cleanupAudioHandler, setupAudioHandler } from "@/handler_audio";
import AudioDeviceStore from "@/stores/audioDeviceStore";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import { loadSfuClient } from "../services/loadSfuClient";
import { getVoiceProviderAdapter } from "../session/providers";
import { runVoiceJoin, VoiceJoinAbortError } from "../session/runVoiceJoin";
import {
	isVoiceInteractive,
	isVoiceLoading,
	type VoicePhase,
	voicePhaseLabel,
} from "../session/voiceSessionTypes";

const RECONNECTING_TOAST_ID = "voice-session-reconnecting";

function showReconnectingToast() {
	// 同 id 重复 showToast 会叠多个实例；优先 update，避免 dismiss 只关掉其中一个
	const updated = updateToast({
		id: RECONNECTING_TOAST_ID,
		content: "正在重连...",
		type: "info",
		duration: false,
	});
	if (updated) return;
	showToast("正在重连...", {
		id: RECONNECTING_TOAST_ID,
		type: "info",
		duration: false,
	});
}

function clearReconnectingToast() {
	// solid-notifications 同 id 可能叠多个实例，dismiss/remove 每次只处理一个
	for (let i = 0; i < 8; i++) {
		try {
			dismissToast({ id: RECONNECTING_TOAST_ID });
		} catch {
			// toast 可能尚未创建或已关闭
		}
		try {
			removeToast({ id: RECONNECTING_TOAST_ID });
		} catch {
			// 没有更多同 id toast
			break;
		}
	}
}

type JoinState = "idle" | "connecting" | "joined" | "reconnecting" | "failed";

type Session = {
	roomName: string;
	client: SFUClient | null;
	signal: AbortSignal;
	status: VoicePhase;
	provider?: SFUProvider;
	error?: string | null;
};

function toLegacyJoinState(phase: VoicePhase): JoinState {
	switch (phase) {
		case "ready":
		case "media_ready":
			return "joined";
		case "reconnecting":
			return "reconnecting";
		case "failed":
			return "failed";
		case "idle":
			return "idle";
		default:
			return "connecting";
	}
}

export function useVoiceSession() {
	const [session, setSession] = createSignal<Session | null>(null);
	const selectedRoom = () => socketStore.selectedRoomInfo();
	let joinAbortController: AbortController | null = null;
	// 活跃 join client 引用，供同房重触发时正确 teardown（setSession 会清 client:null）
	let activeJoinClient: SFUClient | null = null;
	// 幂等守卫：同一 client 的 leaveRoom 并发只执行一次
	const leaving = new WeakSet<SFUClient>();
	// 重连中 client 集合：不依赖 session status 刷新时序，避免 onReconnected 竞态漏清 toast
	const reconnectingClients = new WeakSet<SFUClient>();

	const abortCurrentJoin = () => {
		joinAbortController?.abort();
		joinAbortController = null;
	};

	const tokenQuery = useQuery(() => ({
		queryKey: ["token", selectedRoom()?.name],
		enabled: !!selectedRoom(),
		queryFn: async ({ signal }) => {
			const room = selectedRoom();
			if (!room) {
				throw new Error("room is missing");
			}
			return getJoinToken(
				{
					room: room.name,
					identity: userStore.user()?.name ?? "",
					password: room._password,
				},
				signal,
			);
		},
		retry: false,
		staleTime: 5 * 60 * 1000,
	}));

	const phase = createMemo<VoicePhase>(() => session()?.status ?? "idle");
	// VoiceChat 加载条件：
	// - 通用：phase interactive（ready/media_ready/reconnecting）
	// - WHIP 类：client 挂上即加载（interactiveAfterMedia），不堵信令
	const isJoined = createMemo(() => {
		const s = session();
		if (
			s?.client &&
			s.status !== "failed" &&
			s.status !== "leaving" &&
			s.provider &&
			getVoiceProviderAdapter(s.provider).interactiveAfterMedia
		) {
			return true;
		}
		return isVoiceInteractive(phase());
	});
	const isReconnecting = createMemo(() => phase() === "reconnecting");
	// toast 生命周期以 phase 为准：离开 reconnecting 必关，避免回调漏清
	createEffect(() => {
		if (isReconnecting()) {
			showReconnectingToast();
		} else {
			clearReconnectingToast();
		}
	});
	const isLoading = createMemo(() => !isJoined() && isVoiceLoading(phase()));
	const phaseLabel = createMemo(() => voicePhaseLabel(phase()));
	const joinState = createMemo<JoinState>(() => toLegacyJoinState(phase()));
	const currentRoom = createMemo<string | null>(
		() => session()?.roomName ?? null,
	);
	const sfuClient = createMemo<SFUClient | null>(
		() => session()?.client ?? null,
	);
	const error = createMemo<string | null>(() => session()?.error ?? null);

	const teardownClient = async (client: SFUClient) => {
		if (leaving.has(client)) return;
		leaving.add(client);
		reconnectingClients.delete(client);
		await client.leaveRoom().catch(() => {});
		await client.destroy().catch(() => {});
		if (activeJoinClient === client) {
			activeJoinClient = null;
		}
	};

	const teardownSession = async (sess: Session) => {
		clearReconnectingToast();
		setSession((cur) =>
			cur === sess ? { ...cur, status: "leaving", error: null } : cur,
		);
		if (sess.client) {
			await teardownClient(sess.client);
		} else if (activeJoinClient) {
			await teardownClient(activeJoinClient);
		}
		cleanupAudioHandler();
		await socketStore.leaveRoom(sess.roomName).catch(() => {});
		setSession((cur) => (cur === sess ? null : cur));
		socketStore.clearCurrentRoom();
	};

	createEffect(() => {
		const client = session()?.client ?? null;
		if (!selectedRoom()) {
			abortCurrentJoin();
			const toTeardown = client ?? activeJoinClient;
			if (toTeardown) void teardownClient(toTeardown);
			cleanupAudioHandler();
			setSession(null);
			socketStore.clearCurrentRoom();
			socketStore.setCurrentSFUProvider(undefined);
		}
	});

	createEffect(
		on(
			() => tokenQuery.data,
			(data) => {
				if (!data) return;

				abortCurrentJoin();

				const controller = new AbortController();
				joinAbortController = controller;
				const { signal } = controller;

				const provider = resolveSFUProvider(data);
				socketStore.setCurrentSFUProvider(provider);
				const roomPassword = selectedRoom()?._password;
				const audioSettings = AudioDeviceStore.state;
				const newRoom = data.room;
				const prev = session();
				// setSession 会清 client:null，用 activeJoinClient 追踪真实活跃 client
				const prevClient = activeJoinClient;

				// 切房：立即退旧业务房（信令面马上发，不阻塞新 join）。
				// SFU client 拆除仍需串行 await，避免与新房 createOffer 竞态。
				if (prev && prev.roomName !== newRoom) {
					void socketStore.leaveRoom(prev.roomName).catch(() => {});
				}

				// 立即把 session 切到新房 resolving，UI 显示新房
				setSession({
					roomName: newRoom,
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
						// 同房重触发时必须先退旧 client，否则 SRS WHIP 报 5020 stream busy。
						if (prevClient) {
							await teardownClient(prevClient);
							cleanupAudioHandler();
							activeJoinClient = null;
						}
						if (signal.aborted) return;

						const { client, provider: joinedProvider } = await runVoiceJoin(
							data,
							{
								loadClient: loadSfuClient,
								setupAudio: setupAudioHandler,
								joinSignalRoom: (room, identity, password) =>
									socketStore.joinRoom(room, identity, password),
								joinSignalSfu: (room, identity, stream) =>
									socketStore.joinRoomSFU(room, identity, stream),
								onPhase: (nextPhase) => {
									if (signal.aborted) return;
									setSession((s) =>
										s && s.roomName === newRoom && s.signal === signal
											? { ...s, status: nextPhase, error: null }
											: s,
									);
								},
								// 通用 media 完成回调：挂 client 到 session。
								// WHIP 类 media_ready 时 VoiceChat/audio bridge 需要 client 已就绪。
								onClientReady: (client, joinedProvider) => {
									if (signal.aborted) return;
									createdClient = client;
									activeJoinClient = client;
									socketStore.setCurrentSFUProvider(joinedProvider);
									const whipReady =
										!!getVoiceProviderAdapter(joinedProvider)
											.interactiveAfterMedia;
									// WHIP 类：client 挂上即 media_ready，VoiceChat 可加载。
									// 非 WHIP：只挂 client，phase 仍由 onPhase 推进。
									setSession((s) =>
										s && s.roomName === newRoom && s.signal === signal
											? {
													...s,
													client,
													provider: joinedProvider,
													...(whipReady
														? { status: "media_ready" as const }
														: {}),
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
							},
						);
						if (signal.aborted) {
							void teardownClient(client);
							return;
						}

						createdClient = client;
						const joinedClient = client;
						activeJoinClient = joinedClient;
						socketStore.setCurrentSFUProvider(joinedProvider);

						// join 全部成功后才挂断连/重连回调，避免连接阶段瞬态误报
						joinedClient.onDisconnected((info) => {
							if (signal.aborted) return;
							const cur = session();
							// 仅当当前 session 仍指向该 client 且处于活跃态才视为异常断连
							if (cur?.client !== joinedClient) return;
							if (
								cur?.status !== "ready" &&
								cur?.status !== "media_ready" &&
								cur?.status !== "reconnecting" &&
								!reconnectingClients.has(joinedClient)
							) {
								return;
							}
							console.warn("[RoomDetail] SFU disconnected unexpectedly", info);
							const disconnectedMsg =
								info?.reason === "DUPLICATE_IDENTITY"
									? "该账号已在其他设备加入此房间"
									: "连接已断开";
							reconnectingClients.delete(joinedClient);
							setSession((s) =>
								s && s.client === joinedClient
									? { ...s, status: "failed", error: disconnectedMsg }
									: s,
							);
							// phase effect 会清 reconnecting toast
							showToast(disconnectedMsg, { type: "error" });
							void teardownSession(cur);
						});
						joinedClient.onReconnecting?.(() => {
							if (signal.aborted) return;
							const cur = session();
							if (cur?.client !== joinedClient) return;
							if (
								cur?.status !== "ready" &&
								cur?.status !== "media_ready" &&
								cur?.status !== "reconnecting"
							) {
								return;
							}
							reconnectingClients.add(joinedClient);
							setSession((s) =>
								s && s.client === joinedClient
									? { ...s, status: "reconnecting", error: null }
									: s,
							);
							// toast 由 phase effect 统一展示
						});
						joinedClient.onReconnected?.(() => {
							if (signal.aborted) return;
							const cur = session();
							if (cur?.client !== joinedClient) return;
							// 用 WeakSet 标记，规避 session status 尚未 flush 时的漏清
							const wasReconnecting =
								reconnectingClients.has(joinedClient) ||
								cur?.status === "reconnecting";
							reconnectingClients.delete(joinedClient);
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
							showToast("已重连", { type: "success" });
						});

						setSession({
							roomName: newRoom,
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
								activeJoinClient = null;
							}
							return;
						}
						console.error(
							"[RoomDetail] createSFUClient / joinRoom failed:",
							err,
						);
						const message =
							err instanceof Error && err.message
								? err.message
								: "加入房间失败";
						setSession((s) =>
							s && s.roomName === newRoom
								? { ...s, status: "failed", error: message, client: null }
								: s,
						);
						showToast("加入房间失败", { type: "error" });
						if (createdClient) {
							void teardownClient(createdClient);
							cleanupAudioHandler();
							if (activeJoinClient === createdClient) {
								activeJoinClient = null;
							}
						}
					}
				})();
			},
			{ defer: false },
		),
	);

	createEffect(() => {
		if (!tokenQuery.isError) return;
		const tokenError = tokenQuery.error as {
			response?: { data?: { msg?: string } };
			message?: string;
		} | null;
		const msg = tokenError?.response?.data?.msg || tokenError?.message || "";
		if (msg.includes("room is full")) {
			showToast("房间已满，无法加入", { type: "error" });
			socketStore.clearSelectedRoom();
			return;
		}
		if (msg) {
			setSession((s) =>
				s
					? { ...s, status: "failed", error: msg }
					: {
							roomName: selectedRoom()?.name ?? "",
							client: null,
							signal: new AbortController().signal,
							status: "failed",
							error: msg,
						},
			);
		}
	});

	const handleManualLeave = async () => {
		abortCurrentJoin();
		const sess = session();
		if (sess) await teardownSession(sess);
		else {
			if (activeJoinClient) {
				await teardownClient(activeJoinClient);
			}
			cleanupAudioHandler();
		}
		// 兜底：teardownSession 失败时 currentRoom 仍可能残留
		socketStore.clearCurrentRoom();
		socketStore.setCurrentSFUProvider(undefined);
		socketStore.clearSelectedRoom();
	};

	// 被踢时复用手动离开逻辑：abort 进行中 join + teardown 媒体 + 清状态
	onMount(() => {
		onCleanup(
			socketStore.onRoomKicked(() => {
				void handleManualLeave();
			}),
		);
	});

	return {
		selectedRoom,
		phase,
		joinState,
		sfuClient,
		isJoined,
		isReconnecting,
		isLoading,
		phaseLabel,
		currentRoom,
		error,
		handleManualLeave,
		retry: () => tokenQuery.refetch(),
		teardownSession,
	};
}
