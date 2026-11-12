import * as ScrollAreaPrimitive from "@radix-ui/react-scroll-area";
import { forwardRef, type ReactNode } from "react";
import { cn } from "../lib/utils";

export interface ScrollAreaProps {
	className?: string;
	/** 滚动内容所在的 viewport，需要自己控制内边距时用 */
	viewportClassName?: string;
	viewportRef?: React.Ref<HTMLDivElement>;
	children?: ReactNode;
	/**
	 * 滚动条显示策略：auto = 有溢出就显示（默认），scroll = 滚动时显示，
	 * hover = 悬停显示，always = 常显。
	 */
	type?: "auto" | "always" | "scroll" | "hover";
	orientation?: "vertical" | "horizontal" | "both";
	dir?: "ltr" | "rtl";
}

const ScrollBar = forwardRef<
	React.ComponentRef<typeof ScrollAreaPrimitive.Scrollbar>,
	React.ComponentPropsWithoutRef<typeof ScrollAreaPrimitive.Scrollbar>
>(({ className, orientation = "vertical", ...props }, ref) => (
	<ScrollAreaPrimitive.Scrollbar
		ref={ref}
		orientation={orientation}
		className={cn(
			"flex touch-none select-none transition-colors duration-150",
			orientation === "vertical" && "h-full w-1.5 p-px",
			orientation === "horizontal" && "h-1.5 flex-col p-px",
			className,
		)}
		{...props}
	>
		<ScrollAreaPrimitive.Thumb className="bg-muted-foreground/35 hover:bg-muted-foreground/60 relative flex-1 rounded-full transition-colors" />
	</ScrollAreaPrimitive.Scrollbar>
));
ScrollBar.displayName = "ScrollAreaScrollBar";

/**
 * 覆盖式滚动区域。
 *
 * 原生 `overflow-y-auto` 在 Windows / Linux 上会占掉 8~15px 布局宽度，
 * 列表一出现滚动条内容就横向抖动；这里用 Radix 的绝对定位滚动条，
 * 只浮在内容之上，不参与布局，所以「滚动条不挤压空间」。
 */
export const ScrollArea = forwardRef<HTMLDivElement, ScrollAreaProps>(
	(
		{
			className,
			viewportClassName,
			viewportRef,
			children,
			type = "auto",
			orientation = "vertical",
			dir,
			...props
		},
		ref,
	) => (
		<ScrollAreaPrimitive.Root
			ref={ref}
			type={type}
			dir={dir}
			className={cn("relative overflow-hidden", className)}
			{...props}
		>
			<ScrollAreaPrimitive.Viewport
				ref={viewportRef}
				className={cn("size-full", viewportClassName)}
			>
				{children}
			</ScrollAreaPrimitive.Viewport>
			{orientation !== "horizontal" && <ScrollBar />}
			{orientation !== "vertical" && <ScrollBar orientation="horizontal" />}
			<ScrollAreaPrimitive.Corner />
		</ScrollAreaPrimitive.Root>
	),
);
ScrollArea.displayName = "ScrollArea";
