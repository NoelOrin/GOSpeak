import clsx from "clsx";
import { createSignal, Show } from "solid-js";
import type { JSX } from "solid-js/jsx-runtime";

interface OptionSquareProps {
	class?: string;
	children: JSX.Element;
	label?: string;
	onClick?: () => void | Promise<void>;
	active?: boolean;
	onDoubleClick?: () => void | Promise<void>;
	requiresDoubleClick?: boolean;
	actionHint?: string;
}

const OptionSquare = (props: OptionSquareProps) => {
	const [tooltip, setTooltip] = createSignal<{ x: number; y: number } | null>(
		null,
	);
	const [tooltipText, setTooltipText] = createSignal<string | undefined>(
		props.label,
	);

	const showTooltip = (event: MouseEvent | FocusEvent, text?: string) => {
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		setTooltipText(text ?? props.label ?? props.actionHint ?? "双击进入");
		setTooltip({
			x: rect.right + 8,
			y: rect.top + rect.height / 2,
		});
	};

	const hideTooltip = () => {
		setTooltip(null);
		setTooltipText(props.label);
	};

	return (
		<div class="relative">
			<button
				type="button"
				onClick={(event) => {
					if (props.requiresDoubleClick) {
						showTooltip(event, props.actionHint ?? "双击进入");
						return;
					}
					props.onClick?.();
				}}
				onDblClick={() => {
					if (props.requiresDoubleClick) props.onDoubleClick?.();
				}}
				onMouseEnter={(event) => showTooltip(event, props.label)}
				onMouseLeave={hideTooltip}
				onFocus={(event) => showTooltip(event, props.label)}
				onBlur={hideTooltip}
				aria-current={props.active ? "true" : undefined}
				aria-label={props.label}
				class={clsx(
					props.class,
					"btn btn-accent p-0 rounded-2xl size-12 select-none dark:text-white",
					props.active
						? "btn-active bg-neutral/15 text-base-content shadow-sm"
						: "",
				)}
			>
				{props.children}
				<Show when={props.active}>
					<span
						aria-hidden="true"
						class="pointer-events-none absolute bottom-0.5 left-1/2 h-1 w-5 -translate-x-1/2 rounded-full bg-neutral"
					/>
				</Show>
			</button>
			<Show when={props.label && tooltip()}>
				<div
					aria-hidden="true"
					class="pointer-events-none whitespace-nowrap rounded-lg bg-neutral px-2.5 py-1 text-xs font-medium text-neutral-content shadow-lg"
					style={{
						position: "fixed",
						"z-index": 999,
						left: `${tooltip()?.x}px`,
						top: `${tooltip()?.y}px`,
						transform: "translateY(-50%)",
					}}
				>
					{tooltipText()}
				</div>
			</Show>
		</div>
	);
};

export default OptionSquare;
