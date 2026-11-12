import { useQuery } from "@tanstack/react-query";
import { getPresetAvatars } from "@/api/user";

interface PresetAvatarsProps {
	/** 当前选中的头像 URL */
	selected: string;
	/** 选中回调 */
	onSelect: (url: string) => void;
}

export default function PresetAvatars({
	selected,
	onSelect,
}: PresetAvatarsProps) {
	const { data, isLoading } = useQuery({
		queryKey: ["preset-avatars"],
		queryFn: getPresetAvatars,
	});

	return (
		<div className="w-full">
			<div className="text-muted-foreground mb-2 text-sm font-medium">
				预设头像
			</div>
			{isLoading ? (
				<span className="text-faint-foreground text-sm">加载中...</span>
			) : data && data.length > 0 ? (
				<div className="grid grid-cols-5 gap-2 sm:grid-cols-6">
					{data.map((url) => (
						<button
							key={url}
							type="button"
							className={`size-11 cursor-pointer rounded-full ring-2 ring-offset-1 ring-offset-background transition hover:scale-105 sm:size-12 ${
								selected === url
									? "ring-foreground/50"
									: "ring-transparent hover:ring-foreground/20"
							}`}
							onClick={() => onSelect(url)}
						>
							<img src={url} alt="预设头像" className="rounded-full" />
						</button>
					))}
				</div>
			) : (
				<div className="text-faint-foreground text-sm">暂无预设头像</div>
			)}
		</div>
	);
}
