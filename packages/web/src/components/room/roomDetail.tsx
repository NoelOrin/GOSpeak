import { useQuery } from "@tanstack/solid-query";
import { createEffect, createSignal, on, onCleanup, Show } from "solid-js";
import apiClient from "@/api/apiClient";
import createRoom, { type RoomReturnType } from "@/hooks/livekit/createRoom";
import { socketStore } from "@/stores/socketStore";
import VoiceChat from "./voiceChat";

const ROOM_NAME = "test-room";

const RoomDetail = ({ ref }: { ref?: HTMLDivElement }) => {
  const [isJoined, setIsJoined] = createSignal(false);

  // 获取访问令牌（不依赖其他查询）
  const tokenQuery = useQuery(() => ({
    queryKey: ["token", ROOM_NAME],
    queryFn: async () => {
      const response = await apiClient.post({
        url: "/api/v1/signal/token",
        data: {
          room: ROOM_NAME,
          identity: `user-${Date.now().toString(36)}`,
        },
      });
      return (response as any).data as {
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
          setRoomIns(
            createRoom({
              token: data.token,
              url: data.serverUrl,
            })
          );
        }
      }
    )
  );

  // 连接 Socket.IO
  createEffect(() => {
    socketStore.connect();
  });

  // 加入房间：同时触发 Socket.IO 和 LiveKit
  const handleJoin = async () => {
    const data = tokenQuery.data;
    if (!data) return;

    // Socket.IO 加入房间
    socketStore.joinRoom(data.room, data.identity);

    // LiveKit 连接
    await roomIns()?.joinRoom();
    setIsJoined(true);
  };

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

  // 清理
  onCleanup(() => {
    handleLeave();
    socketStore.disconnect();
  });

  return (
    <div
      class="flex flex-1 flex-col justify-center items-center w-full h-full select-none"
      ref={ref}
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
            <button class="btn btn-primary" onClick={handleJoin}>
              加入房间
            </button>
          }
        >
          <div class="flex flex-col w-full h-full">
            <div class="flex justify-between items-center px-4 h-12 border-b border-base-300">
              <span class="font-bold">{socketStore.currentRoom()}</span>
              <div class="flex items-center gap-2">
                <span class="text-sm text-base-content/60">
                  {socketStore.members().length} 人在线
                </span>
                <button class="btn btn-sm btn-ghost" onClick={handleLeave}>
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
