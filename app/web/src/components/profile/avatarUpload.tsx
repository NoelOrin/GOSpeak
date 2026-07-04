import { createSignal, Show } from "solid-js";
import { useUpload } from "@/hooks/useUpload";
import { showToast } from "solid-notifications";

interface AvatarUploadProps {
	/** 当前头像 URL */
	currentAvatar: string;
	/** 上传成功回调，传入新头像 URL */
	onUploadSuccess: (url: string) => void;
}

const AvatarUpload = (props: AvatarUploadProps) => {
	const [preview, setPreview] = createSignal<string | null>(null);
	const { upload, uploading } = useUpload("avatars");
	let fileInput!: HTMLInputElement;

	const handleFileSelect = async (e: Event) => {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;

		// 文件大小限制 5MB
		if (file.size > 5 * 1024 * 1024) {
			showToast("头像文件不能超过 5MB", { type: "warning" });
			return;
		}

		// 格式限制
		if (!["image/jpeg", "image/png", "image/gif", "image/webp"].includes(file.type)) {
			showToast("仅支持 JPG/PNG/GIF/WebP 格式", { type: "warning" });
			return;
		}

		// 本地预览
		const reader = new FileReader();
		reader.onload = (ev) => setPreview(ev.target?.result as string);
		reader.readAsDataURL(file);

		// 上传（S3 模式前端直传，本地模式服务器中转）
		try {
			const result = await upload(file);
			props.onUploadSuccess(result.public_url);
			showToast("头像上传成功", { type: "success" });
		} catch (err: any) {
			showToast(err?.message || "头像上传失败", { type: "error" });
			setPreview(null);
		} finally {
			// 重置 input 以便重复选择同一文件
			target.value = "";
		}
	};

	const displaySrc = () => preview() || props.currentAvatar;

	return (
		<div class="flex flex-col items-center gap-2">
			<button
				type="button"
				class="relative group cursor-pointer"
				onClick={() => fileInput.click()}
				disabled={uploading()}
			>
				<Show
					when={displaySrc()}
					fallback={
						<div class="flex justify-center items-center rounded-full size-24 bg-linear-to-br from-primary to-secondary text-primary-content text-3xl font-bold">
							?
						</div>
					}
				>
					<div class="avatar">
						<div class="rounded-full size-24 ring ring-primary ring-offset-2 ring-offset-base-100">
							<img src={displaySrc()} alt="头像预览" />
						</div>
					</div>
				</Show>
				{/* 悬浮遮罩 */}
				<div class="absolute inset-0 flex items-center justify-center rounded-full bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity">
					<Show
						when={!uploading()}
						fallback={<span class="loading loading-spinner loading-md text-white" />}
					>
						<span class="text-white text-sm">更换头像</span>
					</Show>
				</div>
			</button>
			<input
				ref={fileInput}
				type="file"
				accept="image/jpeg,image/png,image/gif,image/webp"
				class="hidden"
				onChange={handleFileSelect}
			/>
		</div>
	);
};

export default AvatarUpload;
