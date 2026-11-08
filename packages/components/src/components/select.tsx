import * as SelectPrimitive from "@radix-ui/react-select";
import { Check, ChevronDown, ChevronUp } from "lucide-react";
import { forwardRef } from "react";
import { cn } from "../lib/utils";

/**
 * 基于 Radix Select 的下拉选择，替代原生 <select>，
 * 让弹窗/表单里的控件在不同浏览器下样式一致。
 *
 * 注意：Radix 不允许空字符串作为 value，空值场景请用哨兵值
 * （如 `__default__`）映射回业务层的空值。
 */
export const Select = SelectPrimitive.Root;
export const SelectValue = SelectPrimitive.Value;

export const SelectTrigger = forwardRef<
	React.ComponentRef<typeof SelectPrimitive.Trigger>,
	React.ComponentPropsWithoutRef<typeof SelectPrimitive.Trigger>
>(({ className, children, ...props }, ref) => (
	<SelectPrimitive.Trigger
		ref={ref}
		className={cn(
			// inline-flex 而非 flex：默认 w-full 撑满，调用方传 w-auto 时可真正按内容收缩
			// [&>span[data-placeholder]] 让未选择态的占位文字变灰，和已选值区分开
			"border-input bg-card text-foreground focus-visible:ring-ring inline-flex h-9 w-full cursor-pointer items-center justify-between gap-2 rounded-sm border px-3 text-sm shadow-sm focus-visible:ring-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 [&>span]:min-w-0 [&>span]:truncate [&>span[data-placeholder]]:text-muted-foreground",
			className,
		)}
		{...props}
	>
		{children}
		<SelectPrimitive.Icon asChild>
			<ChevronDown className="size-4 shrink-0 opacity-50" />
		</SelectPrimitive.Icon>
	</SelectPrimitive.Trigger>
));
SelectTrigger.displayName = "SelectTrigger";

export const SelectContent = forwardRef<
	React.ComponentRef<typeof SelectPrimitive.Content>,
	React.ComponentPropsWithoutRef<typeof SelectPrimitive.Content>
>(
	(
		{
			className,
			children,
			position = "popper",
			// 左对齐 trigger：默认 center 在弹层比输入框宽时会向两侧溢，看起来像“错位”
			align = "start",
			sideOffset = 4,
			...props
		},
		ref,
	) => (
		<SelectPrimitive.Portal>
			<SelectPrimitive.Content
				ref={ref}
				position={position}
				align={align}
				sideOffset={sideOffset}
				collisionPadding={8}
				className={cn(
					// 高度必须跟 Radix 的可用空间变量走：写死 max-h 会让靠近视口底部的下拉溢出被裁、且滚不到。
					// 20rem 只是"菜单再长也别太高"的绝对上限。
					"bg-popover text-popover-foreground border-border z-[100] max-h-[min(var(--radix-select-content-available-height),20rem)] min-w-[var(--radix-select-trigger-width)] max-w-[var(--radix-select-content-available-width)] overflow-hidden rounded-lg border shadow-md",
					className,
				)}
				{...props}
			>
				{/*
				 * Viewport 不要再自设 max-h：Radix 已经给它内联了
				 * position:relative / flex:1 / overflow:auto，高度由 Content 的 max-h 决定。
				 * 另外 Radix 会注入 scrollbar-width:none 隐藏原生滚动条，
				 * 所以必须配上下 ScrollButton，否则长列表看起来“卡住、滚不动”。
				 */}
				<SelectPrimitive.ScrollUpButton className="text-muted-foreground bg-popover flex h-6 cursor-default items-center justify-center">
					<ChevronUp className="size-4" />
				</SelectPrimitive.ScrollUpButton>
				<SelectPrimitive.Viewport className="w-full p-1">
					{children}
				</SelectPrimitive.Viewport>
				<SelectPrimitive.ScrollDownButton className="text-muted-foreground bg-popover flex h-6 cursor-default items-center justify-center">
					<ChevronDown className="size-4" />
				</SelectPrimitive.ScrollDownButton>
			</SelectPrimitive.Content>
		</SelectPrimitive.Portal>
	),
);
SelectContent.displayName = "SelectContent";

export const SelectItem = forwardRef<
	React.ComponentRef<typeof SelectPrimitive.Item>,
	React.ComponentPropsWithoutRef<typeof SelectPrimitive.Item>
>(({ className, children, ...props }, ref) => (
	<SelectPrimitive.Item
		ref={ref}
		className={cn(
			// Radix 用 data-highlighted 标记“鼠标悬停或键盘聚焦”，只用 :focus 会导致鼠标划过没反馈
			"data-[highlighted]:bg-muted relative flex w-full min-w-0 cursor-pointer items-center rounded-sm py-1.5 pr-2 pl-7 text-sm select-none outline-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
			className,
		)}
		{...props}
	>
		<span className="absolute left-1.5 flex size-4 items-center justify-center">
			<SelectPrimitive.ItemIndicator>
				<Check className="size-3.5" />
			</SelectPrimitive.ItemIndicator>
		</span>
		<SelectPrimitive.ItemText className="block min-w-0 flex-1 truncate">
			{children}
		</SelectPrimitive.ItemText>
	</SelectPrimitive.Item>
));
SelectItem.displayName = "SelectItem";
