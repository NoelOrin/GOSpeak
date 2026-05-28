import { useQuery } from "@tanstack/solid-query";
import { createEffect, createSignal, on, onCleanup, Show } from "solid-js";
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
      console.log(response);
      return (response as any).data?.data as {
        token: string;
        serverUrl: string;
        room: string;
        identity: string;
      };
    },
  }));

  const [roomIns, setRoomIns] = createSignal<RoomReturnType | null>(null);

  // Token 获取成功后初始化 LiveKit Room 实例
  createEffect(
    on(
      () => tokenQuery.isSuccess,
      () => {
        const data = tokenQuery.data;
        if (data) {
          const room = createRoom({
            token: data.token,
            url: data.serverUrl,
          });
          setupAudioHandler(room.room);
          setRoomIns(room);
        }
      }
    )
  );

  // 切换房间时重置状态
  createEffect(
    on(
      () => selectedRoom()?.name,
      () => {
        // 如果已加入则先离开
        if (isJoined()) {
          handleLeave();
        }
        setIsJoined(false);
        setRoomIns(null);
      }
    )
  );

  // 加入房间：同时触发 Socket.IO 和 LiveKit
  const handleJoin = async () => {
    const room = selectedRoom();
    const data = tokenQuery.data;
    if (!room || !data) return;

    // 检查房间容量
    if (room.limit > 0 && room.count >= room.limit) {
      console.warn("[Room] 房间已满，无法加入");
      return;
    }

    // Socket.IO 加入房间
    socketStore.joinRoom(data.room, data.identity);

    // LiveKit 连接
    await roomIns()?.joinRoom();
    // LiveKit 连接成功后通知后端广播
    socketStore.joinRoomLiveKit(data.room, data.identity);
    setIsJoined(true);
  };

  // 点击房间列表项时自动加入房间
  createEffect(
    on(
      () => [tokenQuery.isSuccess, roomIns()] as const,
      ([tokenReady, room]) => {
        if (tokenReady && room && !isJoined()) {
          handleJoin();
        }
      }
    )
  );

  // 离开房间
  const handleLeave = () => {
    if (roomIns()) {
      roomIns()?.leaveRoom();
    }
    if (socketStore.currentRoom()) {
      socketStore.leaveRoom(socketStore.currentRoom()!);
    }
    setIsJoined(false);
  };

  // 手动离开（清除选中状态）
  const handleManualLeave = () => {
    handleLeave();
    socketStore.clearSelectedRoom();
  };

  // 清理（不断开 socket，由 roomList 管理连接生命周期）
  onCleanup(() => {
    handleLeave();
  });

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
      </Show>
    </div>
  );
};

export default RoomDetail;
