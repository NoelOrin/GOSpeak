import { useQuery } from "@tanstack/solid-query";
import { createEffect, createSignal, on, onCleanup, Show } from "solid-js";
import { showToast } from "solid-notifications";
import apiClient from "@/api/apiClient";
import createRoom, { type RoomReturnType } from "@/hooks/livekit/createRoom";
import { setupAudioHandler } from "@/handler_audio";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import VoiceChat from "./voiceChat";

const RoomDetail = ({ ref }: { ref?: HTMLDivElement }) => {
  const [isJoined, setIsJoined] = createSignal(false);
  const [isConnecting, setIsConnecting] = createSignal(false);
  const selectedRoom = () => socketStore.selectedRoomInfo();

  const tokenQuery = useQuery(() => ({
    queryKey: ["token", selectedRoom()?.name],
    enabled: !!selectedRoom(),
    queryFn: async () => {
      const room = selectedRoom()!;
      const response = await apiClient.post({
        url: "/api/v1/signal/token",
        data: {
          room: room.name,
          identity: userStore.user()?.name || `user-${Date.now().toString(36)}`,
        },
      });
      return (response as any).data?.data as {
        token: string;
        serverUrl: string;
        room: string;
        identity: string;
      };
    },
    retry: false,
  }));

  // 当前活跃的 Room 实例，确保同一时间只有一个
  const [roomIns, setRoomIns] = createSignal<RoomReturnType | null>(null);

  // 统一离开逻辑：断开 LiveKit + 通知 socket + 重置状态
  const doLeave = async () => {
    const room = roomIns();
    if (room) await room.leaveRoom();
    if (socketStore.currentRoom()) {
      socketStore.leaveRoom(socketStore.currentRoom()!);
    }
    setRoomIns(null);
    setIsJoined(false);
    setIsConnecting(false);
  };

  // Token 变化时：销毁旧 Room，创建新 Room
  createEffect(
    on(
      () => tokenQuery.data,
      (data) => {
        if (!data) return;

        // 销毁旧实例，防止同时存在多个 LiveKit 连接
        doLeave();

        const room = createRoom({
          token: data.token,
          url: data.serverUrl,
          onConnected: () => {
            // LiveKit 已连接且轨道发布完成后，通知 socket 并更新 UI
            socketStore.joinRoomLiveKit(data.room, data.identity);
            setIsJoined(true);
            setIsConnecting(false);
          },
        });
        setupAudioHandler(room.room);
        setRoomIns(room);

        onCleanup(() => {
          doLeave();
        });
      },
      { defer: false }
    )
  );

  // Token 获取失败时检测房间已满
  createEffect(() => {
    if (tokenQuery.isError) {
      const error = tokenQuery.error as any;
      const msg = error?.response?.data?.msg || "";
      if (msg.includes("room is full")) {
        showToast("房间已满，无法加入", { type: "error" });
        socketStore.clearSelectedRoom();
      }
    }
  });

  // Room 实例就绪后自动加入
  createEffect(
    on(
      roomIns,
      (room) => {
        if (!room || isConnecting()) return;
        const data = tokenQuery.data;
        if (!data) return;

        setIsConnecting(true);
        socketStore.joinRoom(data.room, data.identity);
        room.joinRoom().catch((err) => {
          console.error("[RoomDetail] joinRoom failed:", err);
          setIsConnecting(false);
        });
      },
      { defer: true }
    )
  );

  // 手动离开
  const handleManualLeave = async () => {
    await doLeave();
    socketStore.clearSelectedRoom();
  };

  return (
    <div
      class="flex flex-1 flex-col justify-center items-center w-full h-full select-none"
      ref={ref}
    >
      <Show
        when={selectedRoom()}
        fallback={
          <div class="text-base-content/40 text-sm">
            请从左侧列表选择一个房间
          </div>
        }
      >
        <Show
          when={isJoined()}
          fallback={
            <div class="flex flex-col items-center gap-4">
              <div class="text-lg font-bold">{selectedRoom()?.name}</div>
              <div class="loading loading-spinner loading-sm" />
              <div class="text-sm text-base-content/40">正在加入...</div>
            </div>
          }
        >
          <div class="flex flex-col w-full h-full">
            <div class="flex justify-between items-center px-4 h-12 border-b border-base-300">
              <span class="font-bold">{socketStore.currentRoom()}</span>
              <div class="flex items-center gap-2">
                <span class="text-sm text-base-content/60">
                  {socketStore.members().length} 人在线
                </span>
                <button class="btn btn-sm btn-ghost" onClick={handleManualLeave}>
                  离开
                </button>
              </div>
            </div>
            <VoiceChat />
          </div>
        </Show>
      </Show>
    </div>
  );
};

export default RoomDetail;
