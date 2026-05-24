import {
	Outlet,
	createRootRouteWithContext,
} from "@tanstack/solid-router";
import { TanStackRouterDevtools } from "@tanstack/solid-router-devtools";

import styleCss from "../styles.css?url";

export const Route = createRootRouteWithContext()({
	head: () => ({
		links: [{ rel: "stylesheet", href: styleCss }],
	}),
	shellComponent: RootComponent,
});

function RootComponent() {
	return (
		<>
			<div class="w-screen h-screen overflow-hidden">
				<Outlet />
			</div>
			<TanStackRouterDevtools />
		</>
	);
}
