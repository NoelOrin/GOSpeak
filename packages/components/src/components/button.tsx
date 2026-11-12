import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { forwardRef } from "react";
import { cn } from "../lib/utils";

/**
 * 按钮尺度与全站控件统一（详见 theme.css 圆角注释）：
 *
 * size     高度     用途
 * xs       28px     表格行内操作
 * sm       32px     工具栏 / 密集列表
 * default  36px     主按钮、表单行、页面操作（与 Input / Select 齐平）
 * lg       44px     登录、注册等强场景
 * icon     36×36    图标按钮
 * icon-xs  28×28    密集图标按钮
 *
 * 圆角固定 rounded-sm（4px），与表单控件同族；不使用胶囊按钮。
 */
const buttonVariants = cva(
	"inline-flex cursor-pointer items-center justify-center whitespace-nowrap rounded-sm text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
	{
		variants: {
			variant: {
				default: "bg-primary text-primary-foreground hover:opacity-90",
				outline: "border border-border bg-card text-foreground hover:bg-muted",
				ghost: "text-foreground hover:bg-muted",
				destructive:
					"bg-destructive text-destructive-foreground hover:opacity-90",
			},
			size: {
				xs: "h-7 gap-1 px-2 text-xs",
				sm: "h-8 gap-1.5 px-3 text-xs",
				default: "h-9 gap-2 px-3",
				lg: "h-11 gap-2 px-4",
				icon: "size-9 p-0",
				"icon-xs": "size-7 p-0",
			},
		},
		defaultVariants: { variant: "default", size: "default" },
	},
);

export interface ButtonProps
	extends React.ButtonHTMLAttributes<HTMLButtonElement>,
		VariantProps<typeof buttonVariants> {
	asChild?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
	({ className, variant, size, asChild = false, ...props }, ref) => {
		const Comp = asChild ? Slot : "button";
		return (
			<Comp
				ref={ref}
				className={cn(buttonVariants({ variant, size }), className)}
				{...props}
			/>
		);
	},
);
Button.displayName = "Button";

export { buttonVariants };
