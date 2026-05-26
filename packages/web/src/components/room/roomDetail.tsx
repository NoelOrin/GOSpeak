import { useQuery } from "@tanstack/solid-query";
import { createEffect, createSignal, on, onCleanup, Show } from "solid-js";
import apiClient from "@/api/apiClient";
import createRoom, { type RoomReturnType } from "@/hooks/livekit/createRoom";
import VoiceChat from "./voiceChat";

const RoomDetail = ({ ref }: { ref?: HTMLDivElement }) => {
  const [isVoiceChatVisible, _setIsVoiceChatVisible] = createSignal(true);

  // 创建房间
  const createRoomQuery = useQuery(() => ({
    queryKey: ["create"],
    queryFn: async () => {
      const response = await apiClient.post({
        url: "/api/v1/room/create",
        data: {
          roomName: "test-room",
        },
      });
      return (response as any).data;
    },
  }));

  // 获取访问令牌
  const tokenQuery = useQuery(() => ({
    queryKey: ["token"],
    queryFn: async () => {
      const response = await apiClient.post({
        url: "/api/v1/signal/token",
        data: {
          roomName: "test-room",
          identity: `user${Date.now()}`,
          canPublish: true,
          canSubscribe: true,
        },
      });
      return (response as any).data.data as { token: string; serverUrl: string };
    },
    enabled: createRoomQuery.isSuccess,
  }));

  const [roomIns, setRoom] = createSignal<RoomReturnType | null>(null);

  createEffect(
    on(
      () => tokenQuery.isSuccess,
      async () => {
        const tokenData = tokenQuery.data;

        if (tokenData) {
          setRoom(
            createRoom({
              token: tokenData.token,
              url: tokenData.serverUrl,
            })
          );
        }
      }
    )
  );

  onCleanup(() => {
    if (roomIns()) {
      roomIns()?.leaveRoom();
    }
  });

  return (
    <div
      class="flex flex-1 justify-center items-center w-full h-full select-none"
      ref={ref}
    >
      <Show when={isVoiceChatVisible}>
        <button class="btn" onClick={async () => await roomIns()?.joinRoom()}>
          连接测试
        </button>
        <VoiceChat />
      </Show>
    </div>
  );
};

export default RoomDetail;
