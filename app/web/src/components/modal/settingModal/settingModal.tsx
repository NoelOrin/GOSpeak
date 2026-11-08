import {
	Button,
	Dialog,
	DialogContent,
	DialogTitle,
} from "@gospeak/components";
import { Mic, Monitor, Palette, User, Volume2, X } from "lucide-react";
import type { ComponentType } from "react";
import { useState } from "react";
import { TABS } from "./tab_item";
import type { SettingTabConfig } from "./tab_item/types";

interface SettingModalProps {
	open: boolean;
	onClose: () => void;
	/** 可选：打开时定位到指定 tab */
	initialTabId?: string;
}

const ICON_MAP: Record<
	SettingTabConfig["icon"],
	ComponentType<{ size?: number; className?: string }>
> = {
	mic: Mic,
	volume: Volume2,
	palette: Palette,
	user: User,
	monitor: Monitor,
};

export default function SettingModal({
	open,
	onClose,
	initialTabId,
}: SettingModalProps) {
	const initialIndex = Math.max(
		0,
		TABS.findIndex((t) => t.id === initialTabId),
	);
	const [activeTab, setActiveTab] = useState(
		initialIndex >= 0 ? initialIndex : 0,
	);

	if (!open) return null;

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			{/* 基础 DialogContent 默认是 grid，必须用 flex 显式覆盖（twMerge 只认同组类名）。
			    否则固定高度下 grid 会拉伸 auto 行，标题栏被撑到半屏高、内容区被压缩。 */}
			<DialogContent
				showClose={false}
				className="flex h-[100dvh] max-w-full flex-col gap-0 overflow-hidden rounded-none p-0 sm:h-[min(90vh,820px)] sm:max-w-7xl sm:rounded-2xl"
			>
				<div className="border-border flex items-center justify-between border-b px-4 py-3">
					<div>
						<DialogTitle className="text-base font-semibold">设置</DialogTitle>
						<div className="text-faint-foreground text-xs">
							{TABS.map((t) => t.label).join("、")}
						</div>
					</div>
					<Button
						variant="ghost"
						className="size-9 rounded-full p-0"
						onClick={onClose}
						aria-label="关闭设置"
					>
						<X size={16} />
					</Button>
				</div>
				<div className="flex min-h-0 flex-1 flex-col overflow-hidden select-none sm:flex-row">
					<aside className="border-border bg-muted/70 flex shrink-0 gap-1 overflow-x-auto border-b p-2 sm:w-44 sm:flex-col sm:border-r md:w-48">
						{TABS.map((tab, index) => {
							const Icon = ICON_MAP[tab.icon];
							return (
								<button
									key={tab.id}
									type="button"
									className={`hover:bg-muted flex h-10 shrink-0 cursor-pointer items-center justify-center gap-1.5 rounded-sm px-3 transition-colors sm:h-11 sm:justify-start sm:gap-2 ${activeTab === index ? "bg-card shadow-sm" : ""}`}
									onClick={() => setActiveTab(index)}
								>
									<Icon className="opacity-80" size={16} />
									<span className="text-xs whitespace-nowrap sm:text-sm">
										{tab.label}
									</span>
								</button>
							);
						})}
					</aside>
					<div className="bg-card relative min-w-0 flex-1 overflow-hidden">
						{TABS.map((tab, index) => (
							<div
								key={tab.id}
								className={`absolute inset-0 overflow-auto ${activeTab !== index ? "hidden" : ""}`}
							>
								<tab.component />
							</div>
						))}
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}
