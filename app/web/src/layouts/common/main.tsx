import type { ReactNode } from "react";

export default function Main({ children }: { children: ReactNode }) {
	return (
		<div className="w-full flex-1 select-none overflow-hidden">
			<div className="relative h-full">{children}</div>
		</div>
	);
}
