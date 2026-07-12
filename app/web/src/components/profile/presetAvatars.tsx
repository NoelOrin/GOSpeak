import { For, Show } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import { getPresetAvatars } from "@/api/user";

interface PresetAvatarsProps {
	/** 当前选中的头像 URL */
	selected: string;
	/** 选中回调 */
	onSelect: (url: string) => void;
}

const PresetAvatars = (props: PresetAvatarsProps) => {
	const query = createQuery(() => ({
		queryKey: ["preset-avatars"],
		queryFn: getPresetAvatars,
	}));

	return (
		<div class="w-full">
			<div class="text-sm font-medium mb-2 opacity-70">预设头像</div>
			<Show
				when={!query.isLoading}
				fallback={<span class="loading loading-dots loading-sm" />}
			>
				<Show
					when={query.data && query.data.length > 0}
					fallback={<div class="text-sm opacity-50">暂无预设头像</div>}
				>
					<div class="grid grid-cols-6 gap-2">
						<For each={query.data}>
							{(url) => (
								<button
									type="button"
									class={`avatar rounded-full size-12 ring-2 ring-offset-1 ring-offset-base-100 cursor-pointer transition-all hover:scale-110 ${
										props.selected === url ? "ring-primary" : "ring-transparent"
									}`}
									onClick={() => props.onSelect(url)}
								>
									<img src={url} alt="预设头像" class="rounded-full" />
								</button>
							)}
						</For>
					</div>
				</Show>
			</Show>
		</div>
	);
};

export default PresetAvatars;
