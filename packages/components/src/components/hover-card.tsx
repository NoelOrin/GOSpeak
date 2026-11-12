import * as HoverCardPrimitive from "@radix-ui/react-hover-card";
import { forwardRef } from "react";
import { cn } from "../lib/utils";

/**
 * 基于 Radix HoverCard 的悬停浮层。
 * 与手写 group-hover 不同：内容可交互（放滑块/按钮不会一移入就消失），
 * 且由 Popper 自动定位、避让视口边缘，键盘 focus 也能打开。
 */
export const HoverCard = HoverCardPrimitive.Root;
export const HoverCardTrigger = HoverCardPrimitive.Trigger;

export const HoverCardContent = forwardRef<
	React.ComponentRef<typeof HoverCardPrimitive.Content>,
	React.ComponentPropsWithoutRef<typeof HoverCardPrimitive.Content>
>(
	(
		{
			className,
			align = "center",
			sideOffset = 8,
			collisionPadding = 8,
			...props
		},
		ref,
	) => (
		<HoverCardPrimitive.Portal>
			<HoverCardPrimitive.Content
				ref={ref}
				align={align}
				sideOffset={sideOffset}
				collisionPadding={collisionPadding}
				className={cn(
					"bg-popover text-popover-foreground border-border z-[100] rounded-lg border shadow-md outline-none",
					className,
				)}
				{...props}
			/>
		</HoverCardPrimitive.Portal>
	),
);
HoverCardContent.displayName = "HoverCardContent";
