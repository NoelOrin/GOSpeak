import {
	Button,
	cn,
	Dialog,
	DialogContent,
	DialogTitle,
} from "@gospeak/components";
import { Info, Mic, Monitor, Palette, User, Volume2, X } from "lucide-react";
import type { ComponentType, KeyboardEvent } from "react";
import { useEffect, useRef, useState } from "react";
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
	info: Info,
};

const tabDomId = (id: string) => `setting-tab-${id}`;
const panelDomId = (id: string) => `setting-panel-${id}`;

export default function SettingModal({
	open,
	onClose,
	initialTabId,
}: SettingModalProps) {
	const [activeTab, setActiveTab] = useState(() =>
		Math.max(
			TABS.findIndex((t) => t.id === initialTabId),
			0,
		),
	);
	const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);

	// 每次打开都按 initialTabId 重新定位：组件常驻挂载，仅靠 useState 初值只会生效一次
	useEffect(() => {
		if (!open || !initialTabId) return;
		const index = TABS.findIndex((t) => t.id === initialTabId);
		if (index >= 0) setActiveTab(index);
	}, [open, initialTabId]);

	if (!open) return null;

	const activate = (index: number, focus = true) => {
		const next = (index + TABS.length) % TABS.length;
		setActiveTab(next);
		if (focus) tabRefs.current[next]?.focus();
	};

	const onTablistKeyDown = (event: KeyboardEvent<HTMLElement>) => {
		switch (event.key) {
			case "ArrowRight":
			case "ArrowDown":
				event.preventDefault();
				activate(activeTab + 1);
				break;
			case "ArrowLeft":
			case "ArrowUp":
				event.preventDefault();
				activate(activeTab - 1);
				break;
			case "Home":
				event.preventDefault();
				activate(0);
				break;
			case "End":
				event.preventDefault();
				activate(TABS.length - 1);
				break;
			default:
				break;
		}
	};

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
					<div
						role="tablist"
						aria-label="设置分区"
						aria-orientation="vertical"
						onKeyDown={onTablistKeyDown}
						className="border-border bg-muted/70 flex shrink-0 gap-1 overflow-x-auto border-b p-2 [scrollbar-width:none] sm:w-44 sm:flex-col sm:border-r md:w-48 [&::-webkit-scrollbar]:hidden"
					>
						{TABS.map((tab, index) => {
							const Icon = ICON_MAP[tab.icon];
							const selected = activeTab === index;
							return (
								<button
									key={tab.id}
									ref={(node) => {
										tabRefs.current[index] = node;
									}}
									id={tabDomId(tab.id)}
									type="button"
									role="tab"
									aria-selected={selected}
									aria-controls={panelDomId(tab.id)}
									tabIndex={selected ? 0 : -1}
									className={cn(
										"relative flex h-10 shrink-0 cursor-pointer items-center justify-center gap-1.5 rounded-sm px-3 transition-colors focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-2 sm:h-11 sm:justify-start sm:gap-2",
										selected
											? "bg-card text-foreground shadow-sm"
											: "text-muted-foreground hover:bg-muted hover:text-foreground",
									)}
									onClick={() => setActiveTab(index)}
								>
									<span
										aria-hidden
										className={cn(
											"bg-foreground absolute top-1/2 left-0 hidden h-5 w-0.5 -translate-y-1/2 rounded-full transition-opacity sm:block",
											selected ? "opacity-100" : "opacity-0",
										)}
									/>
									<Icon className="opacity-80" size={16} />
									<span className="text-xs whitespace-nowrap sm:text-sm">
										{tab.label}
									</span>
								</button>
							);
						})}
					</div>
					<div className="bg-card relative min-w-0 flex-1 overflow-hidden">
						{TABS.map((tab, index) => (
							<div
								key={tab.id}
								id={panelDomId(tab.id)}
								role="tabpanel"
								aria-labelledby={tabDomId(tab.id)}
								className={cn(
									"absolute inset-0 overflow-hidden select-text focus-visible:outline-none",
									activeTab !== index && "hidden",
								)}
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
