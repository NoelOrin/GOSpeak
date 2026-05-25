import { createFileRoute } from "@tanstack/solid-router";

export const Route = createFileRoute("/(app)/")({
  component: RouteComponent,
  staticData: {
    title: "首页",
    icon: "home",
  },
});

function RouteComponent() {
  return <>123</>;
}
