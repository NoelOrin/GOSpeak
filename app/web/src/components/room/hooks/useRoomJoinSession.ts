import type { JoinParams, SFUClient } from "@gospeak/sfu-client/types";
import { useQuery } from "@tanstack/solid-query";
import {
	createEffect,
	createMemo,
	createSignal,
	on,
	onCleanup,
	onMount,
} from "solid-js";
import { showToast } from "solid-notifications";
import { getJoinToken } from "@/api/sfu";
import AudioDeviceStore from "@/stores/audioDeviceStore";
import { type MemberInfo, socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import { loadSfuClient, rememberSfuProvider } from "../services/loadSfuClient";
import { resolveJoinSession } from "../services/sfuSession";

type JoinState = "idle" | "connecting" | "joined" | "reconnecting" | "failed";

// 单一真相源：业务房 + SFU 房绑定到同一 session
type Session = {
	roomName: string;
	client: SFUClient | null;
	signal: AbortSignal;
	status: JoinState;
};

class AbortError extends Error {
	name = "AbortError";
}

// 包装 Promise：abort 触发时立即 reject，底层 op 继续后台自拆
function raceAbort<T>(p: Promise<T>, signal: AbortSignal): Promise<T> {
	if (signal.aborted) return Promise.reject(new AbortError());
	return new Promise<T>((resolve, reject) => {
		const onAbort = () => reject(new AbortError());
		signal.addEventListener("abort", onAbort, { once: true });
		p.then(
			(v) => {
				signal.removeEventListener("abort", onAbort);
				resolve(v);
			},
			(e) => {
				signal.removeEventListener("abort", onAbort);
				reject(e);
			},
		);
	});
}

export function useRoomJoinSession() {
	const [session, setSession] = createSignal<Session | null>(null);
	const selectedRoom = () => socketStore.selectedRoomInfo();
	let joinAbortController: AbortController | null = null;
	// 幂等守卫：同一 client 的 leaveRoom 并发只执行一次
	const leaving = new WeakSet<SFUClient>();

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
	}));

	const isJoined = createMemo(() => {
		const s = session()?.status;
		// reconnecting 期间仍保持 joined 视图，避免 VoiceChat 卸载重建媒体元素
		return s === "joined" || s === "reconnecting";
	});
	const isReconnecting = createMemo(() => session()?.status === "reconnecting");
	const joinState = createMemo<JoinState>(() => session()?.status ?? "idle");
	const currentRoom = createMemo<string | null>(
		() => session()?.roomName ?? null,
	);
	const sfuClient = createMemo<SFUClient | null>(
		() => session()?.client ?? null,
	);

	// 仅拆 SFU client（不动 socket 业务房，不碰 session）
	const teardownClient = async (client: SFUClient) => {
		if (leaving.has(client)) return;
		leaving.add(client);
		await client.leaveRoom().catch(() => {});
		await client.destroy().catch(() => {});
	};

	// 完整拆除 session：SFU client + 业务房 + 清 session + 清成员状态
	const teardownSession = async (sess: Session) => {
		if (sess.client) {
			await teardownClient(sess.client);
		}
		await socketStore.leaveRoom(sess.roomName).catch(() => {});
		setSession((cur) => (cur === sess ? null : cur));
		socketStore.clearCurrentRoom();
	};

	createEffect(() => {
		const client = session()?.client ?? null;
		if (!selectedRoom()) {
			abortCurrentJoin();
			if (client) void teardownClient(client);
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

				const sessionMeta = resolveJoinSession(data);
				rememberSfuProvider(sessionMeta.provider);
				socketStore.setCurrentSFUProvider(sessionMeta.provider);
				const roomPassword = selectedRoom()?._password;
				const audioSettings = AudioDeviceStore.state;
				const newRoom = data.room;
				const prev = session();

				// 切房：立即退旧业务房（信令面马上发，不阻塞新 join）。
				// SFU client 拆除仍需串行 await，避免与新房 createOffer 竞态。
				if (prev && prev.roomName !== newRoom) {
					void socketStore.leaveRoom(prev.roomName).catch(() => {});
				}

				// 立即把 session 切到新房 connecting，UI 显示新房
				setSession({
					roomName: newRoom,
					client: null,
					signal,
					status: "connecting",
				});

				void (async () => {
					let createdClient: SFUClient | null = null;
					try {
						// abort 触发时清理已创建的 client 并短路
						const abortIfCancelled = (client: SFUClient | null): boolean => {
							if (!signal.aborted) return false;
							if (client) void teardownClient(client);
							return true;
						};
						// 串行拆旧 SFU client（保证 createOffer 不竞态）
						if (prev?.client && prev.roomName !== newRoom) {
							await teardownClient(prev.client);
						}
						if (abortIfCancelled(createdClient)) return;

						createdClient = await raceAbort(
							loadSfuClient(sessionMeta.provider, {
								audioCapture: {
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
								socket: socketStore.getSocket(),
							}),
							signal,
						);
						if (abortIfCancelled(createdClient)) return;

						const joinParams: JoinParams = {
							token: data.token,
							serverUrl: sessionMeta.connectTarget,
							identity: data.identity,
							room: data.room,
							stream: data.stream,
							streamToken: data.streamToken,
						};
						await raceAbort(
							createdClient.joinRoom(joinParams),
							signal,
						);
						if (abortIfCancelled(createdClient)) return;

						await raceAbort(
							socketStore.joinRoom(data.room, data.identity, roomPassword),
							signal,
						);
						if (abortIfCancelled(createdClient)) return;

						const ack = await raceAbort(
							socketStore.joinRoomSFU(data.room, data.identity, data.stream),
							signal,
						);
						if (abortIfCancelled(createdClient)) return;
						const members: MemberInfo[] = ack?.members ?? [];
						const peers = members
							.filter((m) => m.identity !== data.identity && m.stream)
							.map((m) => ({
								identity: m.identity,
								stream: m.stream as string,
							}));
						if (peers.length) {
							createdClient.subscribePeers?.(peers);
						}

						// join 全部成功后才挂断连/重连回调，避免连接阶段瞬态误报
						createdClient.onDisconnected(() => {
							if (signal.aborted) return;
							const cur = session();
							// 仅当当前 session 仍指向该 client 且处于活跃态（joined/reconnecting）才视为异常断连
							if (cur?.client !== createdClient) return;
							if (cur?.status !== "joined" && cur?.status !== "reconnecting")
								return;
							console.warn("[RoomDetail] SFU disconnected unexpectedly");
							setSession((s) =>
								s && s.client === createdClient
									? { ...s, status: "failed" }
									: s,
							);
							showToast("连接已断开", { type: "error" });
							void teardownSession(cur);
						});
						createdClient.onReconnecting?.(() => {
							if (signal.aborted) return;
							const cur = session();
							if (cur?.client !== createdClient) return;
							if (cur?.status !== "joined") return;
							setSession((s) =>
								s && s.client === createdClient
									? { ...s, status: "reconnecting" }
									: s,
							);
							showToast("正在重连...", { type: "info" });
						});
						createdClient.onReconnected?.(() => {
							if (signal.aborted) return;
							const cur = session();
							if (cur?.client !== createdClient) return;
							if (cur?.status !== "reconnecting") return;
							setSession((s) =>
								s && s.client === createdClient
									? { ...s, status: "joined" }
									: s,
							);
							showToast("已重连", { type: "success" });
						});

						setSession({
							roomName: newRoom,
							client: createdClient,
							signal,
							status: "joined",
						});
					} catch (err) {
						if (signal.aborted) {
							if (createdClient) void teardownClient(createdClient);
							return;
						}
						console.error(
							"[RoomDetail] createSFUClient / joinRoom failed:",
							err,
						);
						setSession((s) =>
							s && s.roomName === newRoom ? { ...s, status: "failed" } : s,
						);
						showToast("加入房间失败", { type: "error" });
						if (createdClient) {
							void teardownClient(createdClient);
						}
					}
				})();
			},
			{ defer: false },
		),
	);

	createEffect(() => {
		if (!tokenQuery.isError) return;
		const error = tokenQuery.error as any;
		const msg = error?.response?.data?.msg || "";
		if (msg.includes("room is full")) {
			showToast("房间已满，无法加入", { type: "error" });
			socketStore.clearSelectedRoom();
		}
	});

	const handleManualLeave = async () => {
		abortCurrentJoin();
		const sess = session();
		if (sess) await teardownSession(sess);
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
		joinState,
		sfuClient,
		isJoined,
		isReconnecting,
		currentRoom,
		handleManualLeave,
		teardownSession,
	};
}
