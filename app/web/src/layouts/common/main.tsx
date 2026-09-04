import type { JSX } from "solid-js/jsx-runtime";

const Main = ({ children }: { children: JSX.Element }) => {
	return (
		<div class="flex-1 w-full h-full select-none overflow-hidden">
			<div class="relative h-full">{children}</div>
		</div>
	);
};

export default Main;
