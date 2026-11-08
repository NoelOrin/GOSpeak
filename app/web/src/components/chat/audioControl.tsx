import {
	HoverCard,
	HoverCardContent,
	HoverCardTrigger,
	Slider,
	Toggle,
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from "@gospeak/components";
import clsx from "clsx";
import type { ComponentType } from "react";

export interface ControlIconProps {
	className?: string;
	size?: number;
}

interface AudioControlProps {
	name: string;
	className?: string;
	/** 静音态图标 */
	MutedIcon: ComponentType<ControlIconProps>;
	/** 非静音态图标 */
	UnmutedIcon: ComponentType<ControlIconProps>;
	muteLabel: string;
	unmuteLabel: string;
	volumeLabel: string;
	volume: number;
	isMute: boolean;
	onChange: (volume: number) => void;
	onCheck: (isMute: boolean) => void;
	/** 禁用时开关不可点击（如被服务端禁言），鼠标悬停提示 reason。 */
	disabled?: boolean;
	disabledTip?: string;
}

/**
 * 底部用户栏的扬声器 / 麦克风控件。
 *
 * 全部走 Radix：Toggle 管静音双态，HoverCard 管可交互的音量浮层，
 * Tooltip 管禁用原因，Slider 管音量取值。不再有手写 group-hover 弹层，
 * 浮层由 Popper 定位（自动避让视口），键盘 focus / ESC 也有一致行为。
 */
export default function AudioControl({
	name,
	className,
	MutedIcon,
	UnmutedIcon,
	muteLabel,
	unmuteLabel,
	volumeLabel,
	volume,
	isMute,
	onChange,
	onCheck,
	disabled,
	disabledTip,
}: AudioControlProps) {
	const MuteIcon = isMute ? MutedIcon : UnmutedIcon;

	// 禁用态不用原生 disabled：disabled 元素不派发 pointer 事件，
	// Radix Tooltip 就没法提示原因。改用 aria-disabled + 拦截回调。
	const toggle = (
		<Toggle
			name={name}
			pressed={isMute}
			aria-disabled={disabled || undefined}
			aria-label={isMute ? unmuteLabel : muteLabel}
			onPressedChange={(pressed) => {
				if (!disabled) onCheck(pressed);
			}}
			className={clsx(
				"size-10 rounded-sm",
				isMute && "text-destructive bg-destructive/10 hover:bg-destructive/15",
				disabled && "cursor-not-allowed opacity-40 hover:bg-transparent",
			)}
		>
			<MuteIcon size={20} />
		</Toggle>
	);

	return (
		<div className={clsx("flex items-center", className)}>
			{disabled ? (
				<TooltipProvider delayDuration={150}>
					<Tooltip>
						<TooltipTrigger asChild>{toggle}</TooltipTrigger>
						<TooltipContent side="top">
							{disabledTip ?? muteLabel}
						</TooltipContent>
					</Tooltip>
				</TooltipProvider>
			) : (
				<HoverCard openDelay={120} closeDelay={250}>
					<HoverCardTrigger asChild>{toggle}</HoverCardTrigger>
					<HoverCardContent
						side="top"
						align="center"
						className="w-12 px-2 py-3"
					>
						<div className="flex flex-col items-center gap-2">
							<span className="text-muted-foreground text-2xs leading-none tabular-nums">
								{Math.round(volume)}%
							</span>
							<Slider
								orientation="vertical"
								aria-label={volumeLabel}
								value={[volume]}
								max={100}
								step={1}
								className="h-24"
								onValueChange={(value) => onChange(value[0] ?? 0)}
							/>
						</div>
					</HoverCardContent>
				</HoverCard>
			)}
		</div>
	);
}
