import clsx from "clsx";
import { useState, type ReactNode } from "react";

interface OptionSquareProps {
	className?: string;
	children: ReactNode;
	label?: string;
	onClick?: () => void | Promise<void>;
	active?: boolean;
	onDoubleClick?: () => void | Promise<void>;
	requiresDoubleClick?: boolean;
	actionHint?: string;
}

export default function OptionSquare({
	className,
	children,
	label,
	onClick,
	active,
	onDoubleClick,
	requiresDoubleClick,
	actionHint,
}: OptionSquareProps) {
	const [tooltip, setTooltip] = useState<{ x: number; y: number } | null>(null);
	const [tooltipText, setTooltipText] = useState<string | undefined>(label);

	const showTooltip = (
		event: React.MouseEvent | React.FocusEvent,
		text?: string,
	) => {
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		setTooltipText(text ?? label ?? actionHint ?? "双击进入");
		setTooltip({
			x: rect.right + 8,
			y: rect.top + rect.height / 2,
		});
	};

	const hideTooltip = () => {
		setTooltip(null);
		setTooltipText(label);
	};

	return (
		<div className="relative">
			<button
				type="button"
				onClick={(event) => {
					if (requiresDoubleClick) {
						showTooltip(event, actionHint ?? "双击进入");
						return;
					}
					void onClick?.();
				}}
				onDoubleClick={() => {
					if (requiresDoubleClick) void onDoubleClick?.();
				}}
				onMouseEnter={(event) => showTooltip(event, label)}
				onMouseLeave={hideTooltip}
				onFocus={(event) => showTooltip(event, label)}
				onBlur={hideTooltip}
				aria-current={active ? "true" : undefined}
				aria-label={label}
				className={clsx(
					className,
					"bg-accent text-accent-foreground hover:opacity-90 relative flex size-12 cursor-pointer select-none items-center justify-center rounded-xl p-0 transition-opacity",
					active ? "bg-neutral/15 text-foreground shadow-sm" : "",
				)}
			>
				{children}
				{active && (
					<span
						aria-hidden="true"
						className="bg-neutral pointer-events-none absolute bottom-0.5 left-1/2 h-1 w-5 -translate-x-1/2 rounded-full"
					/>
				)}
			</button>
			{label && tooltip && (
				<div
					aria-hidden="true"
					className="bg-neutral text-neutral-foreground pointer-events-none rounded-lg px-2.5 py-1 text-xs font-medium whitespace-nowrap shadow-lg"
					style={{
						position: "fixed",
						zIndex: 999,
						left: `${tooltip.x}px`,
						top: `${tooltip.y}px`,
						transform: "translateY(-50%)",
					}}
				>
					{tooltipText}
				</div>
			)}
		</div>
	);
}
