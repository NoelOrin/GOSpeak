import { Button, toast } from "@gospeak/components";
import { useEffect, useRef, useState } from "react";
import { updateProfile } from "@/api/user";
import ConfirmModal from "@/components/common/ConfirmModal";
import { useUpload } from "@/hooks/useUpload";
import { useUserStore } from "@/stores/userStore";
import { localizedApiMessage } from "@/utils/errorMessages";
import { toastActionError } from "@/utils/toast";
import AvatarCropperModal from "./AvatarCropperModal";
import AvatarEditor from "./AvatarEditor";
import ProfileFields from "./ProfileFields";
import { userRoleLabel } from "./roleLabel";

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
};

type Snapshot = { displayName: string; avatar: string };

export default function ProfileForm({
	variant = "page",
	initialMode = "view",
	showUsername = true,
	showRole = true,
	onSaved,
}: ProfileFormProps) {
	const isCompact = variant === "compact";

	const user = useUserStore((s) => s.user);
	const [mode, setMode] = useState<ProfileFormMode>(initialMode);
	const [displayName, setDisplayName] = useState(
		user?.display_name || user?.name || "",
	);
	const [avatar, setAvatar] = useState(user?.avatar || "");
	const [baseline, setBaseline] = useState<Snapshot>({
		displayName: user?.display_name || user?.name || "",
		avatar: user?.avatar || "",
	});
	const [saving, setSaving] = useState(false);
	const [saveError, setSaveError] = useState("");
	const [preview, setPreview] = useState<string | null>(null);
	const [cropOpen, setCropOpen] = useState(false);
	const [cropFile, setCropFile] = useState<File | null>(null);
	const [confirmDiscard, setConfirmDiscard] = useState(false);
	const { upload, uploading } = useUpload("avatars");
	const fileInputRef = useRef<HTMLInputElement>(null);

	// biome-ignore lint/correctness/useExhaustiveDependencies: 仅在 user 变化时同步表单初值；内部 setter 稳定，重跑会覆盖用户编辑中的内容
	useEffect(() => {
		if (user && mode === "view") {
			setDisplayName(user.display_name || user.name || "");
			setAvatar(user.avatar || "");
			setPreview(null);
		}
	}, [user]);

	// 释放裁切预览的 Object URL，避免反复上传时累积内存
	useEffect(() => {
		if (!preview?.startsWith("blob:")) return;
		return () => URL.revokeObjectURL(preview);
	}, [preview]);

	const snapshot = (): Snapshot => ({
		displayName: user?.display_name || user?.name || "",
		avatar: user?.avatar || "",
	});

	const resetFromUser = () => {
		setDisplayName(user?.display_name || user?.name || "");
		setAvatar(user?.avatar || "");
		setPreview(null);
	};

	const enterEdit = () => {
		resetFromUser();
		setBaseline(snapshot());
		setSaveError("");
		setMode("edit");
	};

	const dirty =
		displayName !== baseline.displayName || avatar !== baseline.avatar;

	const cancelEdit = () => {
		if (dirty) {
			setConfirmDiscard(true);
			return;
		}
		resetFromUser();
		setMode("view");
	};

	const discardChanges = () => {
		setConfirmDiscard(false);
		setSaveError("");
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
			setBaseline({ displayName: name, avatar });
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
	const role = userRoleLabel(user?.role);

	return (
		<div
			className={`flex w-full flex-col gap-5 ${isCompact ? "items-stretch" : ""}`}
		>
			<AvatarEditor
				mode={mode}
				src={displayAvatar}
				name={avatarName}
				size={isCompact ? "compact" : "page"}
				uploading={uploading}
				onPickFile={() => fileInputRef.current?.click()}
				onPresetSelect={handlePresetSelect}
				presetsVisible
				selected={avatar}
				username={user?.name || ""}
				roleLabel={role}
			/>

			<input
				ref={fileInputRef}
				type="file"
				accept="image/jpeg,image/png,image/gif,image/webp"
				className="hidden"
				onChange={() => void handleAvatarUpload()}
			/>

			<AvatarCropperModal
				open={cropOpen}
				file={cropFile}
				onCancel={() => setCropOpen(false)}
				onConfirm={(blob, name) => void handleCropConfirm(blob, name)}
			/>

			<ProfileFields
				mode={mode}
				displayName={displayName}
				onDisplayNameChange={setDisplayName}
				onEnter={() => void handleSave()}
				saving={saving}
				showUsername={showUsername}
				showRole={showRole}
				username={user?.name || ""}
				roleLabel={role}
				isCompact={isCompact}
			/>

			{saveError && mode === "edit" && (
				<p className="text-destructive w-full text-sm" role="alert">
					{saveError}
				</p>
			)}

			<div className="flex w-full flex-wrap gap-2">
				{mode === "view" ? (
					<Button
						type="button"
						variant="outline"
						className={isCompact ? "" : "w-full"}
						onClick={enterEdit}
					>
						编辑资料
					</Button>
				) : (
					<>
						<Button
							type="button"
							variant="ghost"
							className={isCompact ? "" : "flex-1"}
							onClick={cancelEdit}
							disabled={saving}
						>
							取消
						</Button>
						<Button
							type="button"
							variant="outline"
							className={isCompact ? "" : "flex-1"}
							onClick={() => void handleSave()}
							disabled={saving || uploading || !dirty}
						>
							保存
						</Button>
					</>
				)}
			</div>

			<ConfirmModal
				open={confirmDiscard}
				title="放弃修改"
				message="当前有未保存的修改，确定要放弃吗？"
				confirmText="放弃修改"
				cancelText="继续编辑"
				onClose={() => setConfirmDiscard(false)}
				onConfirm={discardChanges}
			/>
		</div>
	);
}
