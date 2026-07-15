import { createQuery } from "@tanstack/solid-query";
import { For, Show } from "solid-js";
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
			<div class="mb-2 text-sm font-medium text-base-content/60">预设头像</div>
			<Show
				when={!query.isLoading}
				fallback={<span class="loading loading-dots loading-sm" />}
			>
				<Show
					when={query.data && query.data.length > 0}
					fallback={
						<div class="text-sm text-base-content/45">暂无预设头像</div>
					}
				>
					<div class="grid grid-cols-5 gap-2 sm:grid-cols-6">
						<For each={query.data}>
							{(url) => (
								<button
									type="button"
									class="avatar size-11 cursor-pointer rounded-full ring-2 ring-offset-1 ring-offset-base-100 transition hover:scale-105 sm:size-12"
									classList={{
										"ring-base-content/50": props.selected === url,
										"ring-transparent hover:ring-base-content/20":
											props.selected !== url,
									}}
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
