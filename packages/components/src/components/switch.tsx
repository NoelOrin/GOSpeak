import * as SwitchPrimitive from "@radix-ui/react-switch";
import { forwardRef } from "react";
import { cn } from "../lib/utils";

/** 基于 Radix Switch 的开关，替代原生 checkbox + CSS 拼出来的样式。 */
export const Switch = forwardRef<
	React.ComponentRef<typeof SwitchPrimitive.Root>,
	React.ComponentPropsWithoutRef<typeof SwitchPrimitive.Root>
>(({ className, ...props }, ref) => (
	<SwitchPrimitive.Root
		ref={ref}
		className={cn(
			"bg-border data-[state=checked]:bg-primary focus-visible:ring-ring inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50",
			className,
		)}
		{...props}
	>
		<SwitchPrimitive.Thumb className="bg-background pointer-events-none block size-4 rounded-full shadow transition-transform data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0.5" />
	</SwitchPrimitive.Root>
));
Switch.displayName = "Switch";
