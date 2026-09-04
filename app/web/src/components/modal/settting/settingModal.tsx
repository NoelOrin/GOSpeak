import Mic from "lucide-solid/icons/mic";
import Palette from "lucide-solid/icons/palette";
import User from "lucide-solid/icons/user";
import Volume2 from "lucide-solid/icons/volume-2";
import X from "lucide-solid/icons/x";
import { type Component, createSignal, For, type JSX } from "solid-js";
import { TABS } from "./tab_item";
import type { SettingTabConfig } from "./tab_item/types";

interface SearchModalProps {
	ref: HTMLDialogElement;
	onClose: () => void;
	/** 可选：打开时定位到指定 tab */
	initialTabId?: string;
}

const ICON_MAP: Record<
	SettingTabConfig["icon"],
	(props: { class?: string; size?: number }) => JSX.Element
> = {
	mic: Mic,
	volume: Volume2,
	palette: Palette,
	user: User,
};

const SettingModal: Component<SearchModalProps> = (props) => {
	return (
		<dialog ref={props.ref} class="modal modal-bottom sm:modal-middle z-50">
			<div class="modal-box flex h-[100dvh] sm:h-[min(90vh,820px)] w-full max-w-full sm:max-w-7xl flex-col overflow-hidden p-0 rounded-none sm:rounded-2xl m-0 sm:m-auto z-50">
				<div class="flex items-center justify-between border-b border-base-300 px-4 py-3">
					<div>
						<div class="text-base font-semibold">设置</div>
						<div class="text-xs text-base-content/50">
							音频、语音行为、外观与账户
						</div>
					</div>
					<button
						type="button"
						class="btn btn-sm btn-circle btn-ghost"
						onClick={props.onClose}
						aria-label="关闭设置"
					>
						<X size={16} />
					</button>
				</div>
				<div class="min-h-0 flex-1">
					<SettingContext initialTabId={props.initialTabId} />
				</div>
			</div>

			<form method="dialog" class="modal-backdrop z-40">
				<button type="submit" aria-label="关闭" />
			</form>
		</dialog>
	);
};

export default SettingModal;

const SettingContext = (props: { initialTabId?: string }) => {
	const initialIndex = Math.max(
		0,
		TABS.findIndex((t) => t.id === props.initialTabId),
	);
	const [activeTab, setActiveTab] = createSignal(
		initialIndex >= 0 ? initialIndex : 0,
	);

	return (
		<div class="flex h-full w-full flex-col sm:flex-row overflow-hidden select-none">
			<aside class="flex sm:w-44 md:w-48 shrink-0 flex-row sm:flex-col gap-1 border-b sm:border-b-0 sm:border-r border-base-300 bg-base-200/70 p-2 overflow-x-auto">
				<For each={TABS}>
					{(tab, index) => {
						const Icon = ICON_MAP[tab.icon];
						return (
							<button
								type="button"
								class="btn btn-ghost h-10 sm:h-11 justify-center sm:justify-start gap-1.5 sm:gap-2 px-3 shrink-0"
								classList={{
									"bg-base-100 shadow-sm": activeTab() === index(),
								}}
								onClick={() => setActiveTab(index())}
							>
								<Icon class="opacity-80" size={16} />
								<span class="text-xs sm:text-sm whitespace-nowrap">
									{tab.label}
								</span>
							</button>
						);
					}}
				</For>
			</aside>

			<div class="relative min-w-0 flex-1 overflow-hidden bg-base-100">
				<For each={TABS}>
					{(tab, index) => (
						<div
							class="absolute inset-0 overflow-auto"
							classList={{ hidden: activeTab() !== index() }}
						>
							<tab.component />
						</div>
					)}
				</For>
			</div>
		</div>
	);
};
