import { Camera } from "lucide-react";
import Avatar from "@/components/common/avatar";
import PresetAvatars from "./presetAvatars";

export type AvatarEditorProps = {
	mode: "view" | "edit";
	src: string;
	name: string;
	size: "compact" | "page";
	uploading: boolean;
	onPickFile: () => void;
	onPresetSelect: (url: string) => void;
	presetsVisible: boolean;
	selected: string;
	username?: string;
	roleLabel?: string;
};

export default function AvatarEditor({
	mode,
	src,
	name,
	size,
	uploading,
	onPickFile,
	onPresetSelect,
	presetsVisible,
	selected,
	username,
	roleLabel,
}: AvatarEditorProps) {
	const isCompact = size === "compact";
	const avatarSize = isCompact ? "size-16" : "size-24";
	const avatarText = isCompact ? "text-xl" : "text-3xl";
	const avatarClass = `ring-foreground/15 ring-offset-background ring-2 ring-offset-2 ${avatarSize}`;

	return (
		<div className="flex w-full flex-col gap-4">
			<div
				className={
					isCompact
						? "flex w-full items-center gap-4"
						: "flex w-full flex-col items-center gap-4"
				}
			>
				{mode === "view" ? (
					<Avatar
						src={src || undefined}
						name={name}
						alt="头像"
						className={avatarClass}
						textClass={avatarText}
					/>
				) : (
					<button
						type="button"
						className="group relative shrink-0 cursor-pointer disabled:cursor-not-allowed"
						onClick={onPickFile}
						disabled={uploading}
						title="点击上传头像"
						aria-label="更换头像"
					>
						<Avatar
							src={src || undefined}
							name={name}
							alt="头像"
							className={avatarClass}
							textClass={avatarText}
						/>
						<div
							className={`absolute inset-0 flex items-center justify-center rounded-full bg-black/45 transition-opacity ${
								uploading
									? "opacity-100"
									: "opacity-0 [@media(hover:hover)]:group-hover:opacity-100"
							}`}
						>
							{uploading ? (
								<span className="border-primary border-t-primary size-5 animate-spin rounded-full border-2" />
							) : (
								<span className="text-xs text-white sm:text-sm">更换头像</span>
							)}
						</div>
						<span className="bg-primary text-primary-foreground pointer-events-none absolute -right-0.5 -bottom-0.5 rounded-full p-1.5 [@media(hover:hover)]:hidden">
							<Camera size={14} />
						</span>
					</button>
				)}

				{isCompact && (
					<div className="min-w-0 flex-1">
						<div className="truncate text-base font-semibold">{name}</div>
						<div className="text-faint-foreground text-xs">@{username}</div>
						<div className="bg-muted text-foreground mt-1 inline-flex items-center rounded-full px-2 py-0.5 text-xs">
							{roleLabel || "普通用户"}
						</div>
					</div>
				)}
			</div>

			{presetsVisible && mode === "edit" && (
				<div className="w-full">
					<PresetAvatars selected={selected} onSelect={onPresetSelect} />
				</div>
			)}
		</div>
	);
}
