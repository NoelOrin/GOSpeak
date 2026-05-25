import clsx from "clsx";
import type { JSX } from "solid-js/jsx-runtime";
import { Show } from "solid-js";

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
	return (
    <div class="tooltip-right tooltip">
      <Show when={label}>
        <div class="tooltip-content">
          <div class="font-semibold">{label}</div>
        </div>
      </Show>

      <button
        type="button"
        onClick={onClick}
        class={clsx(
          props.class,
          "btn btn-accent p-0 rounded-2xl size-12 select-none dark:text-white"
        )}
      >
        {children}
      </button>
    </div>
  );
};

export default OptionSquare;
