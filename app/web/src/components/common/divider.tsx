import clsx from "clsx";
import type { ReactNode } from "react";

interface DividerProps {
	className?: string;
	children?: ReactNode;
}

export default function Divider({ className, children }: DividerProps) {
	if (!children) {
		return <div className={clsx("bg-border h-px w-full", className)} />;
	}
	return (
		<div
			className={clsx(
				"text-faint-foreground flex items-center gap-2 text-xs",
				className,
			)}
		>
			<span className="bg-border h-px flex-1" />
			{children}
			<span className="bg-border h-px flex-1" />
		</div>
	);
}
