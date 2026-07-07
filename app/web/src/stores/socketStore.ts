import type { SFUProvider } from "@gospeak/sfu-client/types";
import { createMemo, createRoot, createSignal } from "solid-js";
import { showToast } from "solid-notifications";
import { createSocketClient } from "@/socket/client";
import { EVENTS } from "@/socket/events";
import userStore from "@/stores/userStore";

export { EVENTS } from "@/socket/events";

export interface MemberInfo {
	id: string;
	identity: string;
	name: string;
	displayName: string;
	avatar: string;
	isMuted: boolean;
	isMicMuted: boolean;
	joinedAt: number;
	stream?: string;
}

export interface RoomInfo {
	id: number;
	uuid: string;
	name: string;
	hasPassword: boolean;
	description?: string;
	limit: number;
	audioOnly?: boolean;
	allowAudience?: boolean;
	members: MemberInfo[];
	count: number;
	createdAt: number;
	/** @internal 临时传递密码，不从服务器获取 */
	_password?: string;
}

export interface MuteEvent {
	user_id: number;
	permanent: boolean;
	expires_at: string | null;
	reason: string;
}

export interface UnmuteEvent {
	user_id: number;
}

export interface ActivityEvent {
	type: "member_joined" | "member_left" | "room_joined" | "room_left";
	room: string;
	identity?: string;
	timestamp: number;
}

export interface RoomPresenceEvent {
	type: "member_joined" | "member_left";
	room: string;
	identity: string;
	timestamp: number;
}

export const socketStore = createRoot(() => {
	const adapter = createSocketClient();

	const [connected, setConnected] = createSignal(false);
	const [rooms, setRooms] = createSignal<RoomInfo[]>([]);
	const [currentRoom, setCurrentRoom] = createSignal<string | null>(null);
	// members 派生自 rooms[] 中当前房，消除 members/rooms 双源时序竞态
	const members = createMemo<MemberInfo[]>(
		() => rooms().find((r) => r.name === currentRoom())?.members ?? [],
	);
	const [selectedRoomInfo, setSelectedRoomInfo] = createSignal<RoomInfo | null>(
		null,
	);
	const [activeSFUProvider, setActiveSFUProvider] = createSignal<
		SFUProvider | undefined
	>(undefined);
	const [speechRestricted, setSpeechRestricted] = createSignal<boolean>(false);
	const [speechRestrictionInfo, setSpeechRestrictionInfo] = createSignal<{
		permanent: boolean;
		expires_at: string | null;
		reason: string;
	} | null>(null);
	const [connecting, setConnecting] = createSignal(false);

	const producerReadyListeners = new Set<(info: any) => void>();
	const producerClosedListeners = new Set<(info: any) => void>();
	const activityListeners = new Set<(event: ActivityEvent) => void>();
	const presenceListeners = new Set<(event: RoomPresenceEvent) => void>();
	const kickedListeners = new Set<() => void>();

	function emitActivity(event: ActivityEvent) {
		for (const listener of activityListeners) listener(event);
	}

	function emitPresence(event: RoomPresenceEvent) {
		for (const listener of presenceListeners) listener(event);
	}

	// 生命周期回调：注册一次，每次 connect/disconnect/connect_error 都会触发
	adapter.onConnected(() => {
		setConnecting(false);
		setConnected(true);
		console.log("[Socket] connected:", adapter.getSocket()?.id);
		listRooms();
	});

	adapter.onConnectError((err: Error) => {
		setConnecting(false);
		console.error("[Socket] connect_error:", err.message);
		showToast(`连接服务器失败: ${err.message}`, { type: "error" });
	});

	adapter.onDisconnected((reason: string) => {
		setConnecting(false);
		setConnected(false);
		console.log("[Socket] disconnected:", reason);
	});

	function connect() {
		if (adapter.isConnected() || connecting()) return;
		const token = userStore.accessToken();
		if (!token) {
			showToast("请先登录", { type: "warning" });
			return;
		}
		const socketUrl = import.meta.env.VITE_SOCKET_URL || "";
		setConnecting(true);
		adapter.connect(socketUrl, token);

		adapter.onServerEvent(
			EVENTS.ROOM_CREATED,
			(room: RoomInfo & { error?: string }) => {
				console.log("[Socket] room:created", room.name, room.error);
				if (room.error) {
					showToast(`创建房间失败: ${room.error}`, { type: "error" });
					return;
				}
				setRooms((prev) => {
					if (prev.some((r) => r.name === room.name)) return prev;
					return [...prev, room];
				});
			},
		);

		adapter.onServerEvent(EVENTS.ROOM_UPDATED, (room: RoomInfo) => {
			console.log("[Socket] room:updated", room.name);
			// 仅覆盖服务端实际返回的字段（name/hasPassword/members/count/createdAt），
			// 保留 DB 字段（id/uuid/description/limit/audioOnly/allowAudience）不被零值覆盖。
			// members 由 rooms[] 派生，无需单独维护。
			setRooms((prev) =>
				prev.map((r) =>
					r.name === room.name
						? {
								...r,
								name: room.name,
								hasPassword: room.hasPassword,
								members: room.members,
								count: room.count,
								createdAt: room.createdAt ?? r.createdAt,
							}
						: r,
				),
			);
		});

		adapter.onServerEvent(
			EVENTS.MEMBER_JOINED as string,
			(data: {
				room: string;
				identity: string;
				id: string;
				stream?: string;
			}) => {
				console.log("[Socket] member:joined", data.identity);
				// 即时追加 shell 成员，富化字段由后续 room:updated 覆盖。
				setRooms((prev) =>
					prev.map((r) =>
						r.name === data.room
							? {
									...r,
									count: r.count + 1,
									members: r.members.some((m) => m.id === data.id)
										? r.members
										: [
												...r.members,
												{
													id: data.id,
													identity: data.identity,
													name: "",
													displayName: "",
													avatar: "",
													isMuted: false,
													isMicMuted: false,
													joinedAt: Date.now(),
													stream: data.stream,
												},
											],
								}
							: r,
					),
				);
				emitActivity({
					type: "member_joined",
					room: data.room,
					identity: data.identity,
					timestamp: Date.now(),
				});
				emitPresence({
					type: "member_joined",
					room: data.room,
					identity: data.identity,
					timestamp: Date.now(),
				});
			},
		);

		adapter.onServerEvent(
			EVENTS.MEMBER_LEFT as string,
			(data: { room: string; identity: string; id: string }) => {
				console.log("[Socket] member:left", data.identity);
				setRooms((prev) =>
					prev.map((r) =>
						r.name === data.room
							? {
									...r,
									count: Math.max(0, r.count - 1),
									members: r.members.filter((m) => m.id !== data.id),
								}
							: r,
					),
				);
				emitActivity({
					type: "member_left",
					room: data.room,
					identity: data.identity,
					timestamp: Date.now(),
				});
				emitPresence({
					type: "member_left",
					room: data.room,
					identity: data.identity,
					timestamp: Date.now(),
				});
			},
		);

		adapter.onServerEvent(
			EVENTS.MEMBER_UPDATED,
			(data: { room: string; identity: string; isMicMuted: boolean }) => {
				// members 由 rooms[] 派生，仅更新 rooms[] 即可
				setRooms((prev) =>
					prev.map((r) =>
						r.name === data.room
							? {
									...r,
									members: r.members.map((m) =>
										m.identity === data.identity
											? { ...m, isMicMuted: data.isMicMuted }
											: m,
									),
								}
							: r,
					),
				);
			},
		);

		adapter.onServerEvent(
			EVENTS.ROOM_LIST_RESULT as string,
			(data: { rooms: RoomInfo[]; count: number }) => {
				console.log("[Socket] room:list:result", data.rooms.length, "rooms");
				setRooms(data.rooms);
			},
		);

		adapter.onServerEvent(
			EVENTS.ROOM_KICKED,
			(data: { room: string; targetIdentity: string }) => {
				console.log("[Socket] room:kicked", data.room, data.targetIdentity);
				if (data.targetIdentity !== userStore.user()?.name) return;
				showToast("你已被移出房间", { type: "error" });
				setCurrentRoom(null);
				setSelectedRoomInfo(null);
				for (const listener of kickedListeners) listener();
			},
		);

		adapter.onServerEvent(EVENTS.SFU_PRODUCER_READY, (info: any) => {
			for (const listener of producerReadyListeners) listener(info);
		});

		adapter.onServerEvent(EVENTS.SFU_PRODUCER_CLOSED, (info: any) => {
			for (const listener of producerClosedListeners) listener(info);
		});

		// 用户级禁言事件：允许收听，不允许发布本地音轨
		adapter.onServerEvent(EVENTS.USER_MUTED, (data: MuteEvent) => {
			console.log("[Socket] user:muted", data.user_id);
			showToast(
				data.permanent
					? "你已被永久禁言，当前为仅收听模式"
					: `你已被禁言，当前为仅收听模式${data.reason ? `，原因: ${data.reason}` : ""}`,
				{ type: "warning" },
			);
			setSpeechRestricted(true);
			setSpeechRestrictionInfo({
				permanent: data.permanent,
				expires_at: data.expires_at,
				reason: data.reason,
			});
		});

		adapter.onServerEvent(EVENTS.USER_UNMUTED, (data: UnmuteEvent) => {
			console.log("[Socket] user:unmuted", data.user_id);
			showToast("你的禁言已被解除，可以重新发言", { type: "success" });
			setSpeechRestricted(false);
			setSpeechRestrictionInfo(null);
		});
	}

	function disconnect() {
		producerReadyListeners.clear();
		producerClosedListeners.clear();
		activityListeners.clear();
		presenceListeners.clear();
		kickedListeners.clear();
		adapter.offAllServerEvents();
		adapter.disconnect();
		setConnecting(false);
		setConnected(false);
		setCurrentRoom(null);
		setSpeechRestricted(false);
		setSpeechRestrictionInfo(null);
		setActiveSFUProvider(undefined);
	}

	function createRoom(name: string, password?: string) {
		adapter.emitFireAndForget(EVENTS.ROOM_CREATE, { room: name, password });
	}

	function signalEmit(
		event: string,
		payload?: Record<string, unknown>,
	): Promise<any> {
		return adapter.emitAck(event, payload);
	}

	function clearCurrentRoom() {
		setCurrentRoom(null);
	}

	function joinRoom(room: string, identity: string, password?: string) {
		return signalEmit(EVENTS.ROOM_JOIN, { room, identity, password }).then(
			(data) => {
				if (data.error) {
					throw new Error(data.error);
				}
				setCurrentRoom(data.room);
				return data;
			},
		);
	}

	function leaveRoom(room: string): Promise<any> {
		// 仅发信令，不清本地状态。状态清理由 session 生命周期统一管理
		// （teardownSession / handleManualLeave / room:kicked），避免切房 fire-and-forget 时
		// .then 清状态与新房 joinRoom 设状态竞态。
		return signalEmit(EVENTS.ROOM_LEAVE, { room });
	}

	function joinRoomSFU(room: string, identity: string, stream?: string) {
		return signalEmit(EVENTS.ROOM_JOIN_SFU, { room, identity, stream }).then(
			(data) => {
				// ack 返回完整成员列表，即时 upsert rooms[]（不等 room:updated）
				if (data.members) {
					const ackMembers: MemberInfo[] = data.members;
					setRooms((prev) => {
						const exists = prev.some((r) => r.name === data.room);
						if (!exists) {
							return [
								...prev,
								{
									id: 0,
									uuid: "",
									name: data.room,
									hasPassword: false,
									limit: 0,
									members: ackMembers,
									count: ackMembers.length,
									createdAt: Date.now(),
								},
							];
						}
						return prev.map((r) =>
							r.name === data.room
								? { ...r, members: ackMembers, count: ackMembers.length }
								: r,
						);
					});
				}
				emitActivity({
					type: "room_joined",
					room: data.room,
					identity: data.identity,
					timestamp: Date.now(),
				});
				return data;
			},
		);
	}

	function getRouterCapabilities(room: string) {
		return signalEmit(EVENTS.SFU_GET_ROUTER_CAPABILITIES, { room });
	}

	function createTransport(room: string, direction: "send" | "recv") {
		return signalEmit(EVENTS.SFU_CREATE_TRANSPORT, { room, direction });
	}

	function connectTransport(
		room: string,
		transportId: string,
		dtlsParameters: unknown,
	) {
		return signalEmit(EVENTS.SFU_CONNECT_TRANSPORT, {
			room,
			transportId,
			dtlsParameters,
		});
	}

	function produce(
		room: string,
		transportId: string,
		kind: string,
		rtpParameters: unknown,
		appData: unknown,
	) {
		return signalEmit(EVENTS.SFU_PRODUCE, {
			room,
			transportId,
			kind,
			rtpParameters,
			appData,
		});
	}

	function consume(
		room: string,
		transportId: string,
		producerId: string,
		rtpCapabilities: unknown,
	) {
		return signalEmit(EVENTS.SFU_CONSUME, {
			room,
			transportId,
			producerId,
			rtpCapabilities,
		});
	}

	function onProducerReady(cb: (info: any) => void): () => void {
		producerReadyListeners.add(cb);
		return () => {
			producerReadyListeners.delete(cb);
		};
	}

	function onProducerClosed(cb: (info: any) => void): () => void {
		producerClosedListeners.add(cb);
		return () => {
			producerClosedListeners.delete(cb);
		};
	}

	function onActivity(cb: (event: ActivityEvent) => void): () => void {
		activityListeners.add(cb);
		return () => {
			activityListeners.delete(cb);
		};
	}

	function onPresence(cb: (event: RoomPresenceEvent) => void): () => void {
		presenceListeners.add(cb);
		return () => {
			presenceListeners.delete(cb);
		};
	}

	function onRoomKicked(cb: () => void): () => void {
		kickedListeners.add(cb);
		return () => {
			kickedListeners.delete(cb);
		};
	}

	function getSocket() {
		return adapter.getSocket();
	}

	function listRooms() {
		adapter.emitFireAndForget(EVENTS.ROOM_LIST);
	}

	function kickMember(room: string, targetIdentity: string) {
		adapter.emitFireAndForget(EVENTS.ROOM_KICK, {
			room,
			targetIdentity,
		});
	}

	function emitMicState(room: string, identity: string, isMicMuted: boolean) {
		adapter.emitFireAndForget(EVENTS.MEMBER_MIC_STATE, {
			room,
			identity,
			isMicMuted,
		});
	}

	function selectRoom(room: RoomInfo) {
		setSelectedRoomInfo(room);
	}

	function clearSelectedRoom() {
		setSelectedRoomInfo(null);
	}

	function setCurrentSFUProvider(provider?: SFUProvider) {
		setActiveSFUProvider(provider);
	}

	return {
		connected,
		rooms,
		currentRoom,
		members,
		selectedRoomInfo,
		activeSFUProvider,
		speechRestricted,
		speechRestrictionInfo,
		connect,
		disconnect,
		createRoom,
		joinRoom,
		leaveRoom,
		joinRoomSFU,
		clearCurrentRoom,
		getRouterCapabilities,
		createTransport,
		connectTransport,
		produce,
		consume,
		onProducerReady,
		onProducerClosed,
		onActivity,
		onPresence,
		onRoomKicked,
		getSocket,
		listRooms,
		kickMember,
		emitMicState,
		selectRoom,
		clearSelectedRoom,
		setCurrentSFUProvider,
	};
});
