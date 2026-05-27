import SvgIcon from "@/components/svgIcon";
import { For, Show, createSignal, onMount, onCleanup } from "solid-js";
import { socketStore, type MemberInfo } from "@/stores/socketStore";

const VoiceChat = ({ ref }: { ref?: HTMLDivElement }) => {
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
          <For each={socketStore.members()}>
            {(member) => <MemberCard member={member} />}
          </For>
        </Show>
      </div>
    </div>
  );
};

const MemberCard = ({ member }: { member: MemberInfo }) => {
  const initials = member.identity.slice(0, 2).toUpperCase();

  return (
    <div class="box-border relative flex flex-col flex-1 rounded-xl w-full aspect-5/4 select-none">
      <div class="flex justify-center items-center rounded-xl h-full overflow-hidden bg-gradient-to-br from-primary/20 to-secondary/20">
        <div class="flex justify-center items-center w-20 h-20 rounded-full bg-primary/30 text-primary-content text-2xl font-bold">
          {initials}
        </div>
      </div>
      <div class="flex justify-between items-center px-2 py-1">
        <span class="text-sm font-medium truncate">{member.identity}</span>
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
      </div>
    </div>
  );
};

export default VoiceChat;
