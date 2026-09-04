import ChevronDown from "lucide-solid/icons/chevron-down";
import { createSignal, For, Show } from "solid-js";

const GUIDE_ITEMS = [
	{
		title: "创建或加入房间",
		text: "在左侧房间列表双击房间进入，或用首页快捷入口先进入域页再操作。",
	},
	{
		title: "音量与静音",
		text: "底部用户栏和房间内控件可调整输入输出音量，也可以快速切换本地静音状态。",
	},
	{
		title: "退出与切换",
		text: "进入房间后可在右上角离开，返回房间列表后可以继续切换到其他房间。",
	},
	{
		title: "管理入口",
		text: "管理员可以直接进入域管理、禁言和 SFU 配置页面处理日常运营操作。",
	},
] as const;

const OperationGuide = () => {
	const [expanded, setExpanded] = createSignal(true);

	return (
		<section class="rounded-lg border border-base-300 bg-base-100 p-5 shadow-sm">
			<button
				type="button"
				class="flex w-full items-center justify-between gap-4 text-left"
				onClick={() => setExpanded((value) => !value)}
			>
				<div>
					<h2 class="text-lg font-semibold">操作指南</h2>
					<p class="text-sm text-base-content/60">常用流程与快捷说明</p>
				</div>
				<ChevronDown
					size={18}
					class={`transition-transform ${expanded() ? "rotate-180" : "rotate-0"}`}
				/>
			</button>
			<Show when={expanded()}>
				<div class="mt-4 grid gap-3 md:grid-cols-2">
					<For each={GUIDE_ITEMS}>
						{(item) => (
							<div class="rounded-lg border border-base-300/70 bg-base-200/40 px-4 py-4">
								<div class="font-medium">{item.title}</div>
								<div class="mt-2 text-sm leading-6 text-base-content/70">
									{item.text}
								</div>
							</div>
						)}
					</For>
				</div>
			</Show>
		</section>
	);
};

export default OperationGuide;
