import { createRoot, createSignal } from "solid-js";

/** 当前正在发言的成员 identity 列表 */
export const [speakingIdentities, setSpeakingIdentities] = createRoot(() =>
  createSignal<string[]>([]),
);
