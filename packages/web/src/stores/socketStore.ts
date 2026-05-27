import io from "socket.io-client";
import type { Socket } from "socket.io-client";
import { createSignal, createRoot } from "solid-js";

// 事件常量（与后端 signal/events.go 一致）
export const EVENTS = {
  ROOM_CREATE: "room:create",
  ROOM_JOIN: "room:join",
  ROOM_LEAVE: "room:leave",
  ROOM_LIST: "room:list",
  ROOM_CREATED: "room:created",
  ROOM_JOINED: "room:joined",
  ROOM_LEFT: "room:left",
  ROOM_UPDATED: "room:updated",
  MEMBER_JOINED: "member:joined",
  MEMBER_LEFT: "member:left",
  ROOM_LIST_RESULT: "room:list:result",
} as const;

export interface MemberInfo {
  id: string;
  identity: string;
  joinedAt: number;
}

export interface RoomInfo {
  id: number;
  uuid: string;
  name: string;
  limit: number;
  members: MemberInfo[];
  count: number;
  createdAt: number;
}

function createSocketStore() {
  const [connected, setConnected] = createSignal(false);
  const [rooms, setRooms] = createSignal<RoomInfo[]>([]);
  const [currentRoom, setCurrentRoom] = createSignal<string | null>(null);
  const [members, setMembers] = createSignal<MemberInfo[]>([]);
  const [selectedRoomInfo, setSelectedRoomInfo] = createSignal<RoomInfo | null>(null);

  let socket: Socket | null = null;

  function connect() {
    if (socket?.connected) return;
    if (socket) {
      socket.disconnect();
      socket = null;
    }
    const socketUrl = import.meta.env.VITE_SOCKET_URL || "http://localhost:8998";
    socket = io(socketUrl, { transports: ["websocket", "polling"] });

    socket.on("connect", () => {
      setConnected(true);
      console.log("[Socket] connected:", socket?.id);
      console.log("[Socket] transport:", socket.io.engine.transport.name);
      listRooms();
    });

    socket.on("connect_error", (err) => {
      console.error("[Socket] connect_error:", err.message);
    });

    socket.on("disconnect", (reason) => {
      setConnected(false);
      console.log("[Socket] disconnected:", reason);
    });

    socket.on(EVENTS.ROOM_CREATED, (room: RoomInfo) => {
      console.log("[Socket] room:created", room.name);
      setRooms((prev) => {
        if (prev.some((r) => r.name === room.name)) return prev;
        return [...prev, room];
      });
    });

    socket.on(EVENTS.ROOM_UPDATED, (room: RoomInfo) => {
      console.log("[Socket] room:updated", room.name);
      setRooms((prev) => prev.map((r) => (r.name === room.name ? room : r)));
    });

    socket.on(
      EVENTS.ROOM_JOINED,
      (data: { room: string; members: MemberInfo[]; count: number }) => {
        console.log("[Socket] room:joined", data.room, data.members);
        setCurrentRoom(data.room);
        setMembers(data.members);
      }
    );

    socket.on(EVENTS.ROOM_LEFT, (data: { room: string }) => {
      console.log("[Socket] room:left", data.room);
      setCurrentRoom(null);
      setMembers([]);
    });

    socket.on(
      EVENTS.MEMBER_JOINED,
      (data: { room: string; identity: string; id: string }) => {
        console.log("[Socket] member:joined", data.identity);
        setMembers((prev) => {
          if (prev.some((m) => m.id === data.id)) return prev;
          return [
            ...prev,
            { id: data.id, identity: data.identity, joinedAt: Date.now() },
          ];
        });
      }
    );

    socket.on(
      EVENTS.MEMBER_LEFT,
      (data: { room: string; identity: string; id: string }) => {
        console.log("[Socket] member:left", data.identity);
        setMembers((prev) => prev.filter((m) => m.id !== data.id));
      }
    );

    socket.on(
      EVENTS.ROOM_LIST_RESULT,
      (data: { rooms: RoomInfo[]; count: number }) => {
        console.log("[Socket] room:list:result", data.rooms.length, "rooms");
        setRooms(data.rooms);
      }
    );
  }

  function disconnect() {
    if (socket) {
      socket.disconnect();
      socket = null;
    }
    setConnected(false);
    setCurrentRoom(null);
    setMembers([]);
  }

  function createRoom(name: string) {
    socket?.emit(EVENTS.ROOM_CREATE, JSON.stringify({ room: name }));
  }

  function joinRoom(room: string, identity: string) {
    socket?.emit(EVENTS.ROOM_JOIN, JSON.stringify({ room, identity }));
  }

  function leaveRoom(room: string) {
    socket?.emit(EVENTS.ROOM_LEAVE, JSON.stringify({ room }));
  }

  function listRooms() {
    socket?.emit(EVENTS.ROOM_LIST);
  }

  function selectRoom(room: RoomInfo) {
    setSelectedRoomInfo(room);
  }

  function clearSelectedRoom() {
    setSelectedRoomInfo(null);
  }

  return {
    connected,
    rooms,
    currentRoom,
    members,
    selectedRoomInfo,
    connect,
    disconnect,
    createRoom,
    joinRoom,
    leaveRoom,
    listRooms,
    selectRoom,
    clearSelectedRoom,
  };
}

export const socketStore = createRoot(createSocketStore);
