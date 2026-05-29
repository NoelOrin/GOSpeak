import { useQuery } from "@tanstack/solid-query";
import { createEffect, createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import apiClient from "@/api/apiClient";
import createRoom, { type RoomReturnType } from "@/hooks/livekit/createRoom";
import { setupAudioHandler } from "@/handler_audio";
import { socketStore } from "@/stores/socketStore";
import userStore from "@/stores/userStore";
import VoiceChat from "./voiceChat";

const RoomDetail = ({ ref }: { ref?: HTMLDivElement }) => {
  const [isJoined, setIsJoined] = createSignal(false);
  const selectedRoom = () => socketStore.selectedRoomInfo();

  // 获取访问令牌（依赖当前选中的房间）
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

  const [roomIns, setRoomIns] = createSignal<RoomReturnType | null>(null);

  // Token 获取成功后初始化 LiveKit Room 实例
  createEffect(() => {
    const data = tokenQuery.data;
    if (data) {
      const room = createRoom({
        token: data.token,
        url: data.serverUrl,
      });
      setupAudioHandler(room.room);
      setRoomIns(room);
    }
  });

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

  // 加入房间
  const handleJoin = async () => {
    const room = selectedRoom();
    const data = tokenQuery.data;
    if (!room || !data || isJoined()) return;

    socketStore.joinRoom(data.room, data.identity);
    await roomIns()?.joinRoom();
    socketStore.joinRoomLiveKit(data.room, data.identity);
    setIsJoined(true);
  };

  // 离开房间
  const handleLeave = async () => {
    const room = roomIns();
    if (room) {
      await room.leaveRoom();
    }
    if (socketStore.currentRoom()) {
      socketStore.leaveRoom(socketStore.currentRoom()!);
    }
    setIsJoined(false);
  };

  // 防止 leave/join 重入
  let switching = false;

  // 自动加入：token + room 实例就绪时，先离开旧房间再加入新房间
  createEffect(async () => {
    const data = tokenQuery.data;
    const room = roomIns();

    if (!data || !room || switching) return;

    if (isJoined()) {
      switching = true;
      await handleLeave();
      switching = false;
    }

    switching = true;
    await handleJoin();
    switching = false;
  });

  // 手动离开
  const handleManualLeave = async () => {
    await handleLeave();
    socketStore.clearSelectedRoom();
  };

  return (
    <div
      class="flex flex-1 flex-col justify-center items-center w-full h-full select-none bg-base-200"
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
          when={tokenQuery.isSuccess}
          fallback={
            <div class="loading loading-spinner loading-lg">
              {!tokenQuery.isSuccess && "加载中..."}
            </div>
          }
        >
          <Show
            when={isJoined()}
            fallback={
              <div class="flex flex-col items-center gap-4">
                <div class="text-lg font-bold">{selectedRoom()?.name}</div>
                <div class="loading loading-spinner loading-sm"></div>
                <div class="text-sm text-base-content/40">正在加入...</div>
              </div>
            }
          >
            <div class="flex flex-col flex-1 w-full min-h-0">
              <div class="flex justify-between items-center px-4 h-12 border-b border-base-300 shrink-0">
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
      </Show>
    </div>
  );
};

export default RoomDetail;
