import { createFileRoute } from "@tanstack/solid-router";

// 纯占位 避免路由跳到 /index
export const Route = createFileRoute("/(app)/index/")();
