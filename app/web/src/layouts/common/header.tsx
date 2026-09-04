import { useLocation, useRouter } from "@tanstack/solid-router";
import Sun from "lucide-solid/icons/sun";
import { createEffect, createSignal, onMount } from "solid-js";
import store, { initTheme, toggleDark } from "@/stores/themeStore";

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
					<Sun class="size-4" />
				</button>
			</div>
		</div>
	);
};

export default Header;
