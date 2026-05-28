import clsx from "clsx";
import { For, Show, onMount } from "solid-js";
import { socketStore, type RoomInfo } from "@/stores/socketStore";
import { resumeAudioContext } from "@/handler_audio";
import Divider from "../common/divider";

// 子项
interface RoomItemPropsType {
  room: RoomInfo;
  isActive?: boolean;
}

const RoomItem = ({ room, isActive = false }: RoomItemPropsType) => {
  const isSelected = () => socketStore.selectedRoomInfo()?.name === room.name;

  return (
    <div class="flex flex-col w-full">
      <div
        class={clsx(
          "tooltip tooltip-right w-full",
          isSelected() && !isActive ? "tooltip-open" : ""
        )}
        data-tip="双击进入房间"
      >
        <button
          class={clsx(
            "justify-between items-center px-1.5 border-0 btn btn-ghost btn-sm w-full",
            isActive ? "btn-active" : "",
            isSelected() && !isActive ? "bg-base-200" : ""
          )}
          onDblClick={() => {
            resumeAudioContext();
            socketStore.selectRoom(room);
          }}
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
            <span class="text-[14px] leading-0">{room.name}</span>
          </div>

          <div class="flex text-[12px]">
            <div>{room.count}/{room.limit}</div>
          </div>
        </button>
      </div>

      <div class="flex flex-col w-full">
        <For each={room.members}>
          {(member) => (
            <div class="flex items-center px-4 py-0.5 text-xs text-base-content/60">
              <div class="w-1.5 h-1.5 mr-2 rounded-full bg-success" />
              {member.identity}
            </div>
          )}
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
      <button
        class="btn btn-xs btn-ghost"
        onClick={() => socketStore.listRooms()}
      >
        刷新
      </button>
    </div>
  );
};

interface RoomListPropsType {
  ref?: HTMLDivElement;
}

const RoomList = ({ ref }: RoomListPropsType) => {
  // 进入时加载房间列表
  onMount(() => {
    socketStore.connect();
  });

  return (
    <div class="box-border flex flex-col px-2 h-full select-none" ref={ref}>
      <RoomListHeader />
      <Divider class="mx-1 my-1" />
      <div class="relative flex-1 min-h-0">
        <div class="box-border absolute inset-0 flex flex-col space-y-1 overflow-y-auto">
          <Show
            when={socketStore.connected()}
            fallback={<RoomItemSkeleton />}
          >
            <Show
              when={socketStore.rooms().length > 0}
              fallback={
                <div class="flex justify-center items-center h-20 text-xs text-base-content/40">
                  暂无房间
                </div>
              }
            >
              <For each={socketStore.rooms()}>
                {(room) => (
                  <RoomItem
                    room={room}
                    isActive={socketStore.currentRoom() === room.name}
                  />
                )}
              </For>
            </Show>
          </Show>
        </div>
      </div>
    </div>
  );
};

export default RoomList;
