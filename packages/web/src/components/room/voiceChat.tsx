import SvgIcon from "@/components/svgIcon";
import { For, createSignal, onMount, onCleanup } from "solid-js";

const VoiceChat = ({ ref }: { ref?: HTMLDivElement }) => {
  const [columnCount, setColumnCount] = createSignal(4); // 默认列数
  let containerRef: HTMLDivElement | undefined;

  const updateColumnCount = () => {
    if (!containerRef) return;

    const containerWidth = containerRef.clientWidth;
    const minItemWidth = 220; // 每个 item 最小宽度 (px)
    const gap = 16; // gap-4 = 16px

    // 计算最大可能列数
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

    // 初始化列数
    updateColumnCount();

    onCleanup(() => {
      resizeObserver.disconnect();
    });
  });

  const memberList = [1, 2, 5, 12, 4];

  return (
    <div class="relative w-full h-full overflow-y-auto">
      <div
        class="box-border absolute inset-0 justify-center items-center place-content-center gap-2 grid p-4 w-full select-none"
        style={{
          "grid-template-columns": `repeat(auto-fit, minmax(${190}px, 1fr))`,
        }}
        ref={(el) => {
          containerRef = el;
        }}
      >
        <For each={memberList}>
          {(_member) => (
            <>
              <MemberCard />
            </>
          )}
        </For>
      </div>
    </div>
  );
};

const MemberCard = ({ ref }: { ref?: HTMLDivElement }) => {
  return (
    <div
      class="box-border relative flex flex-col flex-1 rounded-xl w-full aspect-5/4 select-none"
      ref={ref}
    >
      <div class="flex justify-center items-center rounded-xl h-full overflow-hidden">
        <img
          src="https://img.daisyui.com/images/profile/demo/distracted1@192.webp"
          alt=""
          class="w-full h-full object-cover"
        />
      </div>
      <div class="right-2 bottom-2 absolute flex justify-center items-center dark:text-white">
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
