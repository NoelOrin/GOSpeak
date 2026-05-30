/** biome-ignore-all assist/source/organizeImports: <1> */
import Main from "@/layouts/common/main";
import Sidebar from "@/layouts/common/sidebar";
import type { JSX } from "solid-js/jsx-runtime";
import Header from "@/layouts/common/header";
import FuncButton from "@/components/funcButton";
import UserBar from "@/components/userBar";
import { Slot, Split } from "cui-solid";
import { createEffect, createSignal, onCleanup } from "solid-js";
import DynamicRender from "@/components/common/dynamicRender";
import Visible from "@/components/common/visible";
import RoomDetail from "@/components/room/roomDetail";
import { useLocation } from "@tanstack/solid-router";

const Layout = ({ children }: { children: JSX.Element }) => {
  const location = useLocation();
  const isChannel = () => location().pathname.startsWith("/channel");
  const [splitWidth, setSplitWidth] = createSignal(
    localStorage.getItem("splitWidth") || "190px",
  );

  const MIN_SPLIT_WIDTH = 333;
  const MAX_SPLIT_WIDTH = 525;

  let prevRef!: HTMLDivElement;
  createEffect(() => {
    
    console.log(splitWidth());
    console.log(isChannel());

    if (!prevRef) return;
    // Observer监听split变化
    const resizeObserver = new ResizeObserver((entries) => {
      for (const _ of entries) {
        const prevWidth = prevRef.offsetWidth;
        // const nextWidth = nextRef?.offsetWidth || 0;
        // console.log(
        //   "Split widths - Prev:",
        //   prevWidth,
        //   "Next:",
        //   nextWidth,
        //   "Total:",
        //   prevWidth + nextWidth
        // );
        setSplitWidth(prevWidth + "px");
        // 保存到localStorage
        localStorage.setItem("splitWidth", prevWidth + "px");
      }
    });
    resizeObserver.observe(prevRef);

    onCleanup(() => {
      resizeObserver.disconnect();
    });
  });

  return (
    <div class="flex flex-col flex-1 h-full">
      <Header />
      <div class="flex flex-1 h-full">
        <div class="flex flex-1 h-full">
          <Split
            min={MIN_SPLIT_WIDTH}
            split={splitWidth()}
            max={MAX_SPLIT_WIDTH}
          >
            <Slot name="prev">
              <div class="flex flex-col justify-between h-full" ref={prevRef}>
                <div class="flex h-full">
                  <Sidebar />
                  <div class="box-border flex-1 border-color border-t border-l border-solid">
                    <DynamicRender />
                  </div>
                </div>
                <UserBar />
              </div>
            </Slot>

            <Slot name="next">
              <Main>
                <div
                  class="flex-1 border-color border-t w-full h-full  bg-base-200"
                  style={{ display: isChannel() ? "none" : undefined }}
                >
                  {children}
                </div>
                <div
                  class="flex-1 border-color border-t w-full h-full  bg-base-200"
                  style={{ display: isChannel() ? undefined : "none" }}
                >
                  <RoomDetail />
                </div>
                <FuncButton />
              </Main>
            </Slot>
          </Split>
        </div>
      </div>
      {/* <Footer /> */}
    </div>
  );
};
export default Layout;
