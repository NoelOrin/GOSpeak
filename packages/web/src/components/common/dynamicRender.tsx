import RoomList from "@/components/room/roomList";
import HomePage from "@/components/home/homePage";
import { createEffect, createMemo, createSignal, on } from "solid-js";
import { Dynamic } from "solid-js/web";
import { useLocation } from "@tanstack/solid-router";

// Define component map using object literal
const COMPONENT_MAP = {
  "/": HomePage,
  "/channel": RoomList,
  // Add other routes here as needed
} as const;

type RoutePath = keyof typeof COMPONENT_MAP;

const DynamicRender = () => {
  const location = useLocation();
  const [_, setCurrentPath] = createSignal<RoutePath>(
    location().pathname as RoutePath
  );

  createEffect(
    on(
      () => location().pathname,
      (newPath: string) => {
        const validPath = Object.keys(COMPONENT_MAP).includes(newPath)
          ? (newPath as RoutePath)
          : "/";
        setCurrentPath(validPath);
      }
    )
  );

  const CurrentComponent = createMemo(() => {
    const path = location().pathname;
    return COMPONENT_MAP[path as RoutePath] || COMPONENT_MAP["/"];
  });
  
  return (
      <Dynamic component={CurrentComponent()} />
  );
};

export default DynamicRender;
