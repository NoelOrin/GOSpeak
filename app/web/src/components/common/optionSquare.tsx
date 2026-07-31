import clsx from "clsx";
import { createSignal, Show } from "solid-js";
import type { JSX } from "solid-js/jsx-runtime";

const OptionSquare = ({
	children,
	label,
	onClick,
	...props
}: {
	class?: string;
	children: JSX.Element;
	label?: string;
	onClick?: () => void | Promise<void>;
}) => {
	const [tooltip, setTooltip] = createSignal<{ x: number; y: number } | null>(
		null,
	);

	const showTooltip = (event: MouseEvent | FocusEvent) => {
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		setTooltip({
			x: rect.right + 8,
			y: rect.top + rect.height / 2,
		});
	};

	const hideTooltip = () => setTooltip(null);

	return (
		<div class="relative">
			<button
				type="button"
				onClick={onClick}
				onMouseEnter={showTooltip}
				onMouseLeave={hideTooltip}
				onFocus={showTooltip}
				onBlur={hideTooltip}
				aria-label={label}
				class={clsx(
					props.class,
					"btn btn-accent p-0 rounded-2xl size-12 select-none dark:text-white",
				)}
			>
				{children}
			</button>
			<Show when={label && tooltip()}>
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
					{label}
				</div>
			</Show>
		</div>
	);
};

export default OptionSquare;
