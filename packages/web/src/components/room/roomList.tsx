import { useQuery } from "@tanstack/solid-query";
import clsx from "clsx";
import { For, Show } from "solid-js";
import type { RoomItemType } from "@/types/room";
import Divider from "../common/divider";
import MemberInfoWindow from "./roomMemberInfo.tsx";

// 子项
interface RoomItemPropsType {
  roomItem: RoomItemType;
  isActive?: boolean; // 可选参数
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
        <For
          each={
            roomItem.memberInfoList || [
              {
                id: 1,
                name: "张三",
                avatar:
                  "https://img.daisyui.com/images/profile/demo/distracted1@192.webp",
              },
            ]
          }
        >
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
// 列表
const RoomList = ({ ref }: RoomListPropsType) => {
  // const { data, isLoading, error, refetch } = useQuery(() => ({
  //   queryKey: ["user"],
  //   queryFn: () => fetch("/api/user/123").then((res) => res.json()),
  //   // 高级配置（useRequest 的所有功能都支持）
  //   // staleTime: 5000, // 5秒内不重新请求（缓存）
  //   // retry: 3, // 失败重试3次
  //   // refetchOnWindowFocus: false,
  // }));
  const isLoading = false;
  const roomItems: RoomItemType[] = [
    { id: 1, name: "房间1", current: 12, limit: 44 },
    { id: 2, name: "房间2", current: 8, limit: 44 },
    { id: 3, name: "房间3", current: 25, limit: 44 },
    { id: 4, name: "房间4", current: 30, limit: 44 },
    { id: 5, name: "房间5", current: 15, limit: 44 },
    { id: 6, name: "房间6", current: 20, limit: 44 },
    { id: 7, name: "房间7", current: 18, limit: 44 },
    { id: 8, name: "房间8", current: 22, limit: 44 },
    { id: 9, name: "房间9", current: 11, limit: 44 },
    { id: 10, name: "房间10", current: 33, limit: 44 },
    { id: 11, name: "房间11", current: 5, limit: 44 },
    { id: 12, name: "房间12", current: 40, limit: 44 },
  ];

  // console.log(roomItems);
  return (
    <div class="box-border flex flex-col px-2 h-full select-none" ref={ref}>
      <RoomListHeader />
      <Divider class="mx-1 my-1" />
      <div class="relative flex-1 min-h-0">
        <div class="box-border absolute inset-0 flex flex-col space-y-1 overflow-y-auto">
          <Show when={isLoading}>
            <For each={Array.from({ length: 12 }, (_, i) => i)}>
              {(_) => <RoomItemSkeleton />}
            </For>
          </Show>
          <Show when={!isLoading}>
            <For each={roomItems}>
              {(roomItem) => <RoomItem roomItem={roomItem} isActive={false} />}
            </For>
          </Show>
        </div>
      </div>
    </div>
  );
};

export default RoomList;
