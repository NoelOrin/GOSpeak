import * as SliderPrimitive from "@radix-ui/react-slider";
import { forwardRef } from "react";
import { cn } from "../lib/utils";

/**
 * 基于 Radix Slider 的滑块。
 *
 * 用 value: number[]（Radix 原生签名），单值场景传 [value] 即可。
 *
 * 水平/垂直都支持：垂直时 Root 必须切成 flex-col 且宽度自适应，否则
 * Track 会按水平方向 grow、Thumb 被挤到轨道外，最终只剩一根细线和一个圆点。
 * Range 的尺寸由 Radix 按方向注入内联样式（水平 width%，垂直 height% + bottom:0），
 * 这里只补另一根轴的 100%，不要写死宽高覆盖它。
 *
 * 颜色全部走项目主题 token，浅色/深色主题自动跟随。
 */
export const Slider = forwardRef<
	React.ComponentRef<typeof SliderPrimitive.Root>,
	React.ComponentPropsWithoutRef<typeof SliderPrimitive.Root>
>(({ className, ...props }, ref) => (
	<SliderPrimitive.Root
		ref={ref}
		className={cn(
			"relative flex w-full touch-none items-center select-none data-[disabled]:opacity-50",
			"data-[orientation=vertical]:w-auto data-[orientation=vertical]:flex-col",
			// 调用方给垂直滑块传 h-* 决定高度，这里不要强制 h-full（会覆盖掉调用方尺寸）
			className,
		)}
		{...props}
	>
		<SliderPrimitive.Track
			className={cn(
				"bg-border relative grow overflow-hidden rounded-full",
				"data-[orientation=horizontal]:h-1.5 data-[orientation=horizontal]:w-full",
				"data-[orientation=vertical]:h-full data-[orientation=vertical]:w-1.5",
			)}
		>
			<SliderPrimitive.Range
				className={cn(
					"bg-primary absolute",
					"data-[orientation=horizontal]:h-full",
					"data-[orientation=vertical]:w-full",
				)}
			/>
		</SliderPrimitive.Track>
		<SliderPrimitive.Thumb className="border-background bg-primary focus-visible:ring-ring block size-4 shrink-0 rounded-full border-2 shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:pointer-events-none" />
	</SliderPrimitive.Root>
));
Slider.displayName = "Slider";
