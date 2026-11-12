import * as TogglePrimitive from "@radix-ui/react-toggle";
import { forwardRef } from "react";
import { cn } from "../lib/utils";

/** 基于 Radix Toggle 的双态按钮：pressed / onPressedChange 对应 data-state=on|off + aria-pressed。 */
export const Toggle = forwardRef<
	React.ComponentRef<typeof TogglePrimitive.Root>,
	React.ComponentPropsWithoutRef<typeof TogglePrimitive.Root>
>(({ className, ...props }, ref) => (
	<TogglePrimitive.Root
		ref={ref}
		className={cn(
			"text-foreground hover:bg-muted focus-visible:ring-ring inline-flex items-center justify-center rounded-lg transition-colors focus-visible:ring-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50",
			className,
		)}
		{...props}
	/>
));
Toggle.displayName = "Toggle";
