import { cn } from "../lib/utils";

export interface BadgeProps {
	/** 未读数量；<= 0 时不渲染。 */
	count?: number;
	/** 超过该值显示 `N+`，默认 99。 */
	max?: number;
	/** 只渲染小圆点，不显示数字。 */
	dot?: boolean;
	className?: string;
	title?: string;
}

/**
 * 通用未读徽章。数字态 / `99+` / dot 三态。
 *
 * 定位由调用方决定（通常绝对定位到图标右上角），组件本身只管外观。
 */
export function Badge({
	count = 0,
	max = 99,
	dot,
	className,
	title,
}: BadgeProps) {
	if (dot) {
		return (
			<span
				data-slot="badge-dot"
				aria-hidden="true"
				className={cn(
					"bg-destructive inline-block size-2 rounded-full",
					className,
				)}
			/>
		);
	}

	if (count <= 0) return null;
	const text = count > max ? `${max}+` : String(count);

	return (
		<span
			data-slot="badge"
			title={title}
			className={cn(
				"bg-primary text-primary-foreground inline-flex min-w-4 items-center justify-center rounded-full px-1 text-2xs font-semibold leading-4",
				className,
			)}
		>
			{text}
		</span>
	);
}
