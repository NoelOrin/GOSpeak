import {
  ErrorComponent,
  RouterProvider,
  createRouter,
} from "@tanstack/solid-router";
import { render } from "solid-js/web";
import { routeTree } from "./routeTree.gen";
import "./styles.css";
import "cui-solid/dist/styles/cui.css";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

export const queryClient = new QueryClient({
	defaultOptions: {
	  queries: {
		// 全局默认配置（可选）
		staleTime: 5 * 60 * 1000, // 5分钟内不重新请求
		retry: 1,
	  },
	},
  });

const router = createRouter({
  routeTree,
  defaultErrorComponent: ({ error }) => <ErrorComponent error={error} />,
  context: {
    queryClient: queryClient,
  },
  defaultPreload: "intent",
  scrollRestoration: true,
  defaultPreloadStaleTime: 0,
});

declare module "@tanstack/solid-router" {
  interface Register {
    router: typeof router;
  }
}

const rootElement = document.getElementById("app");
if (rootElement) {
  render(
    () => (
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    ),
    rootElement
  );
}
