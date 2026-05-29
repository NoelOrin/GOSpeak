import SvgIcon from "@/components/svgIcon";
import { setVolumeByIdentity } from "@/handler_audio";
import { For, Show, createMemo, createSignal, onMount, onCleanup } from "solid-js";
import { socketStore, type MemberInfo } from "@/stores/socketStore";
import userStore from "@/stores/userStore";

const VoiceChat = ({ ref }: { ref?: HTMLDivElement }) => {
  const sortedMembers = createMemo(() => {
    const members = socketStore.members();
    const myName = userStore.user()?.name;
    return [...members].sort((a, b) => {
      if (a.identity === myName) return -1;
      if (b.identity === myName) return 1;
      return 0;
    });
  });
  const [columnCount, setColumnCount] = createSignal(4);
  let containerRef: HTMLDivElement | undefined;

  const updateColumnCount = () => {
    if (!containerRef) return;
    const containerWidth = containerRef.clientWidth;
    const minItemWidth = 220;
    const gap = 16;
    const maxColumns = Math.max(
      1,
      Math.floor((containerWidth + gap) / (minItemWidth + gap))
    );
    setColumnCount(maxColumns);
  };

  onMount(() => {
    if (!containerRef) return;
    const resizeObserver = new ResizeObserver(updateColumnCount);
    resizeObserver.observe(containerRef);
    updateColumnCount();
    onCleanup(() => resizeObserver.disconnect());
  });

  return (
    <div class="relative w-full h-full overflow-y-auto">
      <div
        class="box-border absolute inset-0 justify-center items-center place-content-center gap-2 grid p-4 w-full select-none"
        style={{
          "grid-template-columns": `repeat(auto-fit, minmax(190px, 1fr))`,
        }}
        ref={(el) => {
          containerRef = el;
        }}
      >
        <Show
          when={socketStore.members().length > 0}
          fallback={
            <div class="flex justify-center items-center col-span-full h-32 text-base-content/40">
              暂无成员
            </div>
          }
        >
          <For each={sortedMembers()}>
            {(member) => <MemberCard member={member} />}
          </For>
        </Show>
      </div>
    </div>
  );
};

const MemberCard = ({ member }: { member: MemberInfo }) => {
  const initials = member.identity.slice(0, 2).toUpperCase();
  const [volume, setVolume] = createSignal(100);
  const isMe = () => member.identity === userStore.user()?.name;

  const handleVolume = (e: Event) => {
    const val = Number((e.target as HTMLInputElement).value);
    setVolume(val);
    setVolumeByIdentity(member.identity, val / 100);
  };

  return (
    <div class="box-border relative flex flex-col flex-1 rounded-xl w-full aspect-5/4 select-none">
      <div class="flex justify-center items-center rounded-xl h-full overflow-hidden bg-gradient-to-br from-primary/20 to-secondary/20">
        <div class="flex justify-center items-center w-20 h-20 rounded-full bg-primary/30 text-primary-content text-2xl font-bold">
          {initials}
        </div>
      </div>
      <div class="flex justify-between items-center px-2 py-1">
        <span class="text-sm font-medium truncate">{member.identity}</span>
        <Show when={!isMe()}>
          <div class="dropdown dropdown-end">
            <button class="dark:text-white btn-square btn btn-xs" tabIndex={0}>
              <SvgIcon name="more" />
            </button>
            <div tabIndex="-1" class="z-1 px-0 py-1 w-24 dropdown-content menu">
              <div class="flex flex-col bg-base-100 shadow-sm rounded-lg overflow-hidden join">
                <button type="button" class="list-item">
                  踢出
                </button>
                <button type="button" class="list-item">
                  静音
                </button>
              </div>
            </div>
          </div>
        </Show>
      </div>
      <Show when={!isMe()}>
        <div class="flex items-center gap-1 px-2 pb-1">
          <span class="text-[10px] text-base-content/40">音量</span>
          <input
            type="range"
            min="0"
            max="100"
            value={volume()}
            onInput={handleVolume}
            class="range range-xs range-primary w-full"
          />
        </div>
      </Show>
    </div>
  );
};

export default VoiceChat;