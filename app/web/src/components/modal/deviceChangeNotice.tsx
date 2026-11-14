import {
	Button,
	Checkbox,
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@gospeak/components";
import { HeadphoneOff, MicOff, MoveRight } from "lucide-react";
import { useEffect, useState } from "react";
import { useAudioDeviceStore } from "@/stores/audioDeviceStore";
import DeviceChangeStore, {
	useDeviceChangeStore,
	type DeviceSwap,
} from "@/stores/deviceChangeStore";
import { startDeviceChangeWatch } from "@/stores/deviceChangeStore";

function SwapRow({
	icon,
	title,
	swap,
}: {
	icon: React.ReactNode;
	title: string;
	swap: DeviceSwap;
}) {
	const fromLabel = swap.from
		? swap.from.label || `设备 ${swap.from.id.slice(0, 8)}`
		: "（已移除）";
	return (
		<div className="border-border bg-muted/40 rounded-lg border p-3">
			<div className="text-faint-foreground mb-2 flex items-center gap-1.5 text-xs">
				{icon}
				<span>{title}</span>
			</div>
			<div className="flex flex-wrap items-center gap-2 text-sm">
				<span
					className={
						swap.from
							? "text-muted-foreground line-through"
							: "text-faint-foreground"
					}
				>
					{fromLabel}
				</span>
				<MoveRight size={14} className="text-faint-foreground" />
				<span className="font-medium">
					{swap.to.label || `设备 ${swap.to.id.slice(0, 8)}`}
				</span>
			</div>
		</div>
	);
}

/**
 * 全局设备变化提示：监听 devicechange，输入/输出设备发生增删时弹窗询问是否切换。
 * “以后不再显示”写入 audioDeviceStore.deviceChangeNotice，可在设置 → 音频里改回。
 */
export default function DeviceChangeNotice() {
	const pending = useDeviceChangeStore((s) => s.pending);
	const [dontAskAgain, setDontAskAgain] = useState(false);

	useEffect(() => startDeviceChangeWatch(), []);

	// 每次新提示都把勾选状态归零，避免上一次的选择被继承
	useEffect(() => {
		if (pending) setDontAskAgain(false);
	}, [pending]);

	const close = (accept: boolean) => {
		// 勾选后本次仍按点击的按钮处理，只是下次不再询问
		if (dontAskAgain)
			useAudioDeviceStore.getState().setDeviceChangeNotice(false);
		if (accept) DeviceChangeStore.confirm();
		else DeviceChangeStore.cancel();
	};

	if (!pending) return null;

	return (
		<Dialog open onOpenChange={(v) => !v && close(false)}>
			<DialogContent
				className="z-[60] max-w-md gap-0 overflow-hidden p-0"
				showClose={false}
			>
				<div className="border-border border-b px-6 py-5">
					<DialogHeader>
						<DialogTitle>检测到音频设备变化</DialogTitle>
					</DialogHeader>
					<DialogDescription className="mt-1">
						继续使用新设备会自动切换，也可以保持当前设置。
					</DialogDescription>
				</div>

				<div className="flex flex-col gap-2 px-6 py-5">
					{pending.inputs && (
						<SwapRow
							icon={<MicOff size={14} />}
							title="输入设备（麦克风）"
							swap={pending.inputs}
						/>
					)}
					{pending.outputs && (
						<SwapRow
							icon={<HeadphoneOff size={14} />}
							title="输出设备（扬声器）"
							swap={pending.outputs}
						/>
					)}
					{pending.inputs && (
						<p className="text-faint-foreground text-xs">
							房间内切换会立即生效，无需重新进入。
						</p>
					)}
				</div>

				<div className="border-border border-t px-6 py-4">
					<label className="mb-4 flex cursor-pointer items-center gap-2 text-sm">
						<Checkbox
							checked={dontAskAgain}
							onCheckedChange={(v) => setDontAskAgain(v === true)}
							aria-label="以后不再显示"
						/>
						<span className="text-muted-foreground">
							以后不再显示（可在设置 → 音频中恢复）
						</span>
					</label>
					<div className="flex justify-end gap-2">
						<Button
							type="button"
							variant="outline"
							onClick={() => close(false)}
						>
							取消
						</Button>
						<Button type="button" onClick={() => close(true)}>
							确认切换
						</Button>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}
