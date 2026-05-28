import store, { toggleDark, initTheme } from "@/stores/themeStore";
import { createEffect, createSignal, onMount } from "solid-js";
import { useLocation, useRouter } from "@tanstack/solid-router";

const Header = () => {
  const router = useRouter();
  const location = useLocation();
  const [title, setTitle] = createSignal("默认标题");

  onMount(() => {
    initTheme();
  });

  createEffect(() => {
    const currentPath = location().pathname;
    const matchedRoutes = router.matchRoutes(currentPath);
    const lastMatch = matchedRoutes.at(-1);
    if (lastMatch) {
      const routeTitle = (lastMatch.staticData as any)?.title || "NOT FOUND";
      setTitle(routeTitle);
    }
  });

  return (
    <div class="flex justify-between items-center px-2 py-0.5 overflow-hidden dark:text-white text-xs select-none">
      <div class="flex-1 font-semibold" />
      <div class="flex justify-center items-center">{title()}</div>
      <div class="flex flex-1 justify-end items-center">
        <button
          class="flex items-center gap-1 btn btn-ghost btn-xs"
          onClick={() => toggleDark()}
        >
          <span class="text-xs">{store.theme}</span>
          <svg
            class="fill-current size-4"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
          >
            <path d="M12 2.25a.75.75 0 01.75.75v2.25a.75.75 0 01-1.5 0V3a.75.75 0 01.75-.75zM7.5 12a4.5 4.5 0 119 0 4.5 4.5 0 01-9 0zM18.894 6.166a.75.75 0 00-1.06-1.06l-1.591 1.59a.75.75 0 101.06 1.061l1.591-1.59zM21.75 12a.75.75 0 01-.75.75h-2.25a.75.75 0 010-1.5H21a.75.75 0 01.75.75zM17.834 18.894a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 10-1.061 1.06l1.59 1.591zM12 18a.75.75 0 01.75.75V21a.75.75 0 01-1.5 0v-2.25A.75.75 0 0112 18zM7.758 17.303a.75.75 0 00-1.061-1.06l-1.591 1.59a.75.75 0 001.06 1.061l1.591-1.59zM6 12a.75.75 0 01-.75.75H3a.75.75 0 010-1.5h2.25A.75.75 0 016 12zM6.697 7.757a.75.75 0 001.06-1.06l-1.59-1.591a.75.75 0 00-1.061 1.06l1.59 1.591z" />
          </svg>
        </button>
      </div>
    </div>
  );
};

export default Header;
