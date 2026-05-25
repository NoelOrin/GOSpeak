import { createRootRouteWithContext, Outlet } from "@tanstack/solid-router";
// import { TanStackRouterDevtools } from "@tanstack/solid-router-devtools";
import ContextProvider from "@/layouts/container/ContextProvider";
import type { QueryClient } from "@tanstack/solid-query";
// import { SolidQueryDevtools } from "@tanstack/solid-query-devtools";
// import { TanStackRouterDevtools } from "@tanstack/solid-router-devtools";

export const Route = createRootRouteWithContext<{
  queryClient: QueryClient;
}>()({
  shellComponent: RootComponent,
  // 全局错误边界
  errorComponent: ({ error, reset }) => (
    <div class="flex justify-center items-center w-screen h-screen">
      <div class="text-red-500">
        <h1>发生错误：{error.message}</h1>
        <button onClick={reset}>重试</button>
      </div>
    </div>
  ),
});

function RootComponent() {
  return (
    <ContextProvider>
      <div class="w-screen h-screen overflow-hidden">
        <Outlet />
      </div>
      {/* <SolidQueryDevtools buttonPosition="bottom-right" /> */}
      {/* <TanStackRouterDevtools position="top-right" /> */}
    </ContextProvider>
  );
}
