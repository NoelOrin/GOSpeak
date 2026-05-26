import { useQuery } from "@tanstack/solid-query";
import clsx from "clsx";
import { For, Show } from "solid-js";
import type { RoomItemType } from "@/types/room";
import apiClient from "@/api/apiClient";
import Divider from "../common/divider";
import MemberInfoWindow from "./roomMemberInfo.tsx";

// 子项
interface RoomItemPropsType {
  roomItem: RoomItemType;
  isActive?: boolean;
}

const RoomItem = ({ roomItem, isActive = false }: RoomItemPropsType) => {
  return (
    <div class="flex flex-col w-full">
      <button
        class={clsx(
          "justify-between items-center px-1.5 border-0 btn btn-ghost btn-sm",
          isActive ? "btn-active" : ""
        )}
      >
        <div class="flex items-center space-x-1">
          <span>
            <svg
              width="16"
              height="16"
              viewBox="0 0 48 48"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
            >
              <rect
                x="17"
                y="4"
                width="14"
                height="27"
                rx="7"
                fill="none"
                stroke="currentColor"
                stroke-width="3"
                stroke-linejoin="round"
              />
              <path
                d="M9 23C9 31.2843 15.7157 38 24 38C32.2843 38 39 31.2843 39 23"
                stroke="currentColor"
                stroke-width="3"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <path
                d="M24 38V44"
                stroke="currentColor"
                stroke-width="3"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </span>
          <span class="text-[14px] leading-0">{roomItem.name}</span>
        </div>

        <div class="flex text-[12px]">
          <div>
            {roomItem.current}/{roomItem.limit}
          </div>
        </div>
      </button>

      <div class="flex flex-col w-full">
        <For each={roomItem.memberInfoList ?? []}>
          {(memberInfo) => <MemberInfoWindow memberInfo={memberInfo} />}
        </For>
      </div>
    </div>
  );
};

const RoomItemSkeleton = () => {
  return (
    <div class="flex flex-col w-full">
      <button class="justify-between items-center px-1.5 border-0 btn btn-ghost btn-sm">
        <div class="flex items-center space-x-1 w-full">
          <div class="flex-1 h-4 skeleton"></div>
        </div>

        <div class="flex items-center text-[12px]">
          <div class="w-8 h-4 skeleton"></div>
        </div>
      </button>
    </div>
  );
};

const RoomListHeader = () => {
  return (
    <div class="flex justify-between mt-2">
      <h3 class="font-bold">服务器</h3>
    </div>
  );
};

interface RoomListPropsType {
  ref?: HTMLDivElement;
}

const RoomList = ({ ref }: RoomListPropsType) => {
  const roomListQuery = useQuery(() => ({
    queryKey: ["roomList"],
    queryFn: async () => {
      const response = await apiClient.get({
        url: "/api/v1/room/list",
        params: { page: 1, page_size: 50 },
      });
      return ((response as any).data.data?.list ?? []) as RoomItemType[];
    },
  }));

  return (
    <div class="box-border flex flex-col px-2 h-full select-none" ref={ref}>
      <RoomListHeader />
      <Divider class="mx-1 my-1" />
      <div class="relative flex-1 min-h-0">
        <div class="box-border absolute inset-0 flex flex-col space-y-1 overflow-y-auto">
          <Show when={roomListQuery.isLoading}>
            <For each={Array.from({ length: 5 }, (_, i) => i)}>
              {(_) => <RoomItemSkeleton />}
            </For>
          </Show>
          <Show when={!roomListQuery.isLoading}>
            <For each={roomListQuery.data ?? []}>
              {(roomItem) => <RoomItem roomItem={roomItem} isActive={false} />}
            </For>
          </Show>
        </div>
      </div>
    </div>
  );
};

export default RoomList;
