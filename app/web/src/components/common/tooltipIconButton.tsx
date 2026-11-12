import {
	Button,
	type ButtonProps,
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from "@gospeak/components";
import type { ReactNode } from "react";

export interface TooltipIconButtonProps
	extends Omit<ButtonProps, "size" | "variant" | "children"> {
	/** 操作名称：同时作为 tooltip 文案与无障碍 label。 */
	label: string;
	/** tooltip 出现方向，默认顶部。 */
	side?: "top" | "right" | "bottom" | "left";
	children: ReactNode;
}

/**
 * 列表头部的统一操作按钮：28×28 的 ghost 图标按钮 + Radix tooltip。
 *
 * 之前「新建 / 刷新 / 更多」混用了文本按钮、14px 与 16px 图标和原生 title
 * 提示（延迟长、样式与主题不一致）。所有面板操作统一走这里：
 * 同一尺寸、同一图标字号（16）、同一套 tooltip 交互。
 */
export default function TooltipIconButton({
	label,
	side = "top",
	children,
	...props
}: TooltipIconButtonProps) {
	return (
		<TooltipProvider delayDuration={150}>
			<Tooltip>
				<TooltipTrigger asChild>
					<Button
						variant="ghost"
						size="sm"
						className="size-7 p-0"
						aria-label={label}
						{...props}
					>
						{children}
					</Button>
				</TooltipTrigger>
				<TooltipContent side={side}>{label}</TooltipContent>
			</Tooltip>
		</TooltipProvider>
	);
}
