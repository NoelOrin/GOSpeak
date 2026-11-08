import { toast } from "@gospeak/components";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { updateProfile } from "@/api/user";
import Avatar from "@/components/common/avatar";
import { useUpload } from "@/hooks/useUpload";
import { useUserStore } from "@/stores/userStore";
import { localizedApiMessage } from "@/utils/errorMessages";
import { toastActionError } from "@/utils/toast";
import AvatarCropperModal from "./AvatarCropperModal";
import PresetAvatars from "./presetAvatars";

export type ProfileFormMode = "view" | "edit";

export type ProfileFormProps = {
	/** page: 资料页居中布局；compact: 设置弹窗紧凑布局 */
	variant?: "page" | "compact";
	/** 初始模式 */
	initialMode?: ProfileFormMode;
	/** 是否显示用户名 */
	showUsername?: boolean;
	/** 是否显示角色 */
	showRole?: boolean;
	/** 保存成功后回调 */
	onSaved?: () => void;
	/** 底部额外内容（如退出登录） */
	footer?: ReactNode;
};

const inputClass =
	"border-input bg-card text-foreground focus-visible:ring-ring flex h-10 w-full rounded-sm border px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50";

export default function ProfileForm({
	variant = "page",
	initialMode = "view",
	showUsername = true,
	showRole = true,
	onSaved,
	footer,
}: ProfileFormProps) {
	const isCompact = variant === "compact";

	const user = useUserStore((s) => s.user);
	const [mode, setMode] = useState<ProfileFormMode>(initialMode);
	const [displayName, setDisplayName] = useState(
		user?.display_name || user?.name || "",
	);
	const [avatar, setAvatar] = useState(user?.avatar || "");
	const [saving, setSaving] = useState(false);
	const [saveError, setSaveError] = useState("");
	const [preview, setPreview] = useState<string | null>(null);
	const [cropOpen, setCropOpen] = useState(false);
	const [cropFile, setCropFile] = useState<File | null>(null);
	const { upload, uploading } = useUpload("avatars");
	const fileInputRef = useRef<HTMLInputElement>(null);

	useEffect(() => {
		if (user && mode === "view") {
			setDisplayName(user.display_name || user.name || "");
			setAvatar(user.avatar || "");
			setPreview(null);
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [user]);

	// 释放裁切预览的 Object URL，避免反复上传时累积内存
	useEffect(() => {
		if (!preview?.startsWith("blob:")) return;
		return () => URL.revokeObjectURL(preview);
	}, [preview]);

	const resetFromUser = () => {
		setDisplayName(user?.display_name || user?.name || "");
		setAvatar(user?.avatar || "");
		setPreview(null);
	};

	const enterEdit = () => {
		resetFromUser();
		setMode("edit");
	};

	const cancelEdit = () => {
		resetFromUser();
		setMode("view");
	};

	const handleAvatarUpload = async () => {
		const target = fileInputRef.current;
		const file = target?.files?.[0];
		if (!file) return;

		if (file.size > 5 * 1024 * 1024) {
			toast.warning("头像文件不能超过 5MB");
			target.value = "";
			return;
		}
		if (
			!["image/jpeg", "image/png", "image/gif", "image/webp"].includes(
				file.type,
			)
		) {
			toast.warning("仅支持 JPG/PNG/GIF/WebP 格式");
			target.value = "";
			return;
		}

		// 先打开裁切弹窗，确认后再上传裁切后的图片。
		setCropFile(file);
		setCropOpen(true);
		target.value = "";
	};

	const handleCropConfirm = async (blob: Blob, fileName: string) => {
		setCropOpen(false);
		const croppedFile = new File([blob], fileName, { type: "image/png" });
		const previewUrl = URL.createObjectURL(blob);
		setPreview(previewUrl);
		try {
			const result = await upload(croppedFile);
			setAvatar(result.public_url);
			toast.success("头像上传成功");
		} catch (e) {
			setPreview(null);
			toastActionError(e, "头像上传失败");
		}
	};

	const handlePresetSelect = (url: string) => {
		setPreview(url);
		setAvatar(url);
	};

	const handleSave = async () => {
		const name = displayName.trim();
		if (!name) {
			toast.warning("昵称不能为空");
			return;
		}

		setSaving(true);
		setSaveError("");
		try {
			await updateProfile({ display_name: name, avatar });
			await useUserStore.getState().fetchProfile();
			toast.success("资料已保存");
			setMode("view");
			onSaved?.();
		} catch (e) {
			// 之前这里是空 catch：保存失败只在拦截器闪一条 toast，
			// 用户还停在编辑态、不知道到底存没存上。
			setSaveError(localizedApiMessage(e, "保存失败，请稍后重试"));
		} finally {
			setSaving(false);
		}
	};

	const displayAvatar = preview || avatar;
	const avatarName = displayName || user?.display_name || user?.name || "?";
	const avatarSize = isCompact ? "size-16" : "size-24";
	const avatarText = isCompact ? "text-xl" : "text-3xl";

	const btnBase =
		"inline-flex h-10 cursor-pointer items-center justify-center gap-2 rounded-sm px-4 text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50";

	return (
		<div
			className={`flex w-full flex-col gap-5 ${isCompact ? "items-stretch" : "mx-auto max-w-lg items-center p-6"}`}
		>
			{/* 头像 */}
			<div
				className={`flex w-full gap-4 ${isCompact ? "items-center" : "flex-col items-center"}`}
			>
				{mode === "view" ? (
					<Avatar
						src={displayAvatar || undefined}
						name={avatarName}
						alt="头像"
						className={`ring-foreground/15 ring-offset-background ring-2 ring-offset-2 ${avatarSize}`}
						textClass={avatarText}
					/>
				) : (
					<>
						<button
							type="button"
							className="group relative shrink-0 cursor-pointer"
							onClick={() => fileInputRef.current?.click()}
							disabled={uploading}
							title="点击上传头像"
						>
							<Avatar
								src={displayAvatar || undefined}
								name={avatarName}
								alt="头像"
								className={`ring-foreground/15 ring-offset-background ring-2 ring-offset-2 ${avatarSize}`}
								textClass={avatarText}
							/>
							<div className="absolute inset-0 flex items-center justify-center rounded-full bg-black/45 opacity-0 transition-opacity group-hover:opacity-100">
								{uploading ? (
									<span className="border-primary border-t-primary size-5 animate-spin rounded-full border-2" />
								) : (
									<span className="text-xs text-white sm:text-sm">
										更换头像
									</span>
								)}
							</div>
						</button>
						<input
							ref={fileInputRef}
							type="file"
							accept="image/jpeg,image/png,image/gif,image/webp"
							className="hidden"
							onChange={() => void handleAvatarUpload()}
						/>
					</>
				)}

				<AvatarCropperModal
					open={cropOpen}
					file={cropFile}
					onCancel={() => setCropOpen(false)}
					onConfirm={(blob, name) => void handleCropConfirm(blob, name)}
				/>

				{isCompact && (
					<div className="min-w-0 flex-1">
						<div className="truncate text-base font-semibold">{avatarName}</div>
						<div className="text-faint-foreground text-xs">@{user?.name}</div>
						<div className="bg-muted text-foreground mt-1 inline-flex items-center rounded-full px-2 py-0.5 text-xs">
							{user?.role || "user"}
						</div>
					</div>
				)}
			</div>

			{/* 预设头像 */}
			{mode === "edit" && (
				<div className="w-full">
					<PresetAvatars selected={avatar} onSelect={handlePresetSelect} />
				</div>
			)}

			{/* 字段 */}
			<div className="flex w-full flex-col gap-3">
				{showUsername && (
					<label className="w-full">
						<span className="mb-1 block text-sm">用户名</span>
						<input
							type="text"
							className={inputClass}
							value={user?.name || ""}
							disabled
						/>
						<p className="text-faint-foreground mt-1 text-xs">用户名不可修改</p>
					</label>
				)}

				<label className="w-full">
					<span className="mb-1 block text-sm">昵称</span>
					{mode === "view" ? (
						<div className="bg-muted flex w-full cursor-default rounded-sm px-3 py-2 text-sm">
							{displayName || "—"}
						</div>
					) : (
						<input
							type="text"
							className={inputClass}
							value={displayName}
							placeholder="请输入昵称"
							maxLength={32}
							onChange={(e) => setDisplayName(e.currentTarget.value)}
							onKeyDown={(e) => {
								if (
									e.key === "Enter" &&
									!e.nativeEvent.isComposing &&
									!saving
								) {
									e.preventDefault();
									void handleSave();
								}
							}}
						/>
					)}
				</label>

				{showRole && !isCompact && (
					<label className="w-full">
						<span className="mb-1 block text-sm">角色</span>
						<input
							type="text"
							className={inputClass}
							value={user?.role || ""}
							disabled
						/>
					</label>
				)}
			</div>

			{/* 操作 */}
			{saveError && mode === "edit" && (
				<p className="text-destructive w-full text-sm" role="alert">
					{saveError}
				</p>
			)}
			<div className="flex w-full flex-wrap gap-2">
				{mode === "view" ? (
					<button
						type="button"
						className={`border-border hover:bg-muted ${btnBase} ${isCompact ? "" : "w-full"}`}
						onClick={enterEdit}
					>
						编辑资料
					</button>
				) : (
					<>
						<button
							type="button"
							className={`hover:bg-muted ${btnBase} ${isCompact ? "" : "flex-1"}`}
							onClick={cancelEdit}
							disabled={saving}
						>
							取消
						</button>
						<button
							type="button"
							className={`border-border hover:bg-muted ${btnBase} ${isCompact ? "" : "flex-1"}`}
							onClick={() => void handleSave()}
							disabled={saving || uploading}
						>
							保存
						</button>
					</>
				)}
			</div>

			{footer && <div className="w-full">{footer}</div>}
		</div>
	);
}
