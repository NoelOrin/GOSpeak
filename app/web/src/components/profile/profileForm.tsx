import { createSignal, Show, Switch, Match, createEffect } from "solid-js";
import { showToast } from "solid-notifications";
import userStore from "@/stores/userStore";
import { updateProfile } from "@/api/user";
import { useUpload } from "@/hooks/useUpload";
import PresetAvatars from "./presetAvatars";
import type { UserInfo } from "@/stores/userStore";

type ViewMode = "view" | "edit";

const ProfileForm = () => {
	const user = () => userStore.user();
	const [mode, setMode] = createSignal<ViewMode>("view");
	const [displayName, setDisplayName] = createSignal(user()?.display_name || "");
	const [avatar, setAvatar] = createSignal(user()?.avatar || "");
	const [saving, setSaving] = createSignal(false);
	const [preview, setPreview] = createSignal<string | null>(null);
	const { upload, uploading } = useUpload("avatars");
	let fileInput!: HTMLInputElement;

	// 当 user 数据更新时同步
	createEffect(() => {
		const u = user();
		if (u && mode() === "view") {
			setDisplayName(u.display_name || "");
			setAvatar(u.avatar || "");
			setPreview(null);
		}
	});

	const enterEdit = () => {
		setDisplayName(user()?.display_name || "");
		setAvatar(user()?.avatar || "");
		setPreview(null);
		setMode("edit");
	};

	const cancelEdit = () => {
		setMode("view");
		setPreview(null);
	};

	const handleAvatarUpload = async () => {
		const target = fileInput;
		const file = target.files?.[0];
		if (!file) return;

		if (file.size > 5 * 1024 * 1024) {
			showToast("头像文件不能超过 5MB", { type: "warning" });
			return;
		}
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
			setAvatar(result.public_url);
			showToast("头像上传成功", { type: "success" });
		} catch (err: any) {
			showToast(err?.message || "头像上传失败", { type: "error" });
			setPreview(null);
		} finally {
			target.value = "";
		}
	};

	const handlePresetSelect = (url: string) => {
		setPreview(url);
		setAvatar(url);
	};

	const handleSave = async () => {
		const name = displayName().trim();
		if (!name) {
			showToast("昵称不能为空", { type: "warning" });
			return;
		}

		setSaving(true);
		try {
			await updateProfile({ display_name: name, avatar: avatar() });
			await userStore.fetchProfile();
			showToast("资料已保存", { type: "success" });
			setMode("view");
		} catch (err: any) {
			showToast(err?.message || "保存失败", { type: "error" });
		} finally {
			setSaving(false);
		}
	};

	const displayAvatar = () => preview() || avatar();
	const initial = () => (user()?.display_name || user()?.name || "?").charAt(0).toUpperCase();

	return (
		<div class="flex flex-col items-center gap-6 p-6 w-full max-w-lg mx-auto">
			{/* 头像区域 */}
			<Switch>
				<Match when={mode() === "view"}>
					<Show
						when={displayAvatar()}
						fallback={
							<div class="flex justify-center items-center rounded-full size-24 bg-linear-to-br from-primary to-secondary text-primary-content text-3xl font-bold">
								{initial()}
							</div>
						}
					>
						<div class="avatar">
							<div class="rounded-full size-24 ring ring-primary ring-offset-2 ring-offset-base-100">
								<img src={displayAvatar()} alt="头像" />
							</div>
						</div>
					</Show>
				</Match>
				<Match when={mode() === "edit"}>
					<button
						type="button"
						class="relative group cursor-pointer"
						onClick={() => fileInput.click()}
						disabled={uploading()}
					>
						<Show
							when={displayAvatar()}
							fallback={
								<div class="flex justify-center items-center rounded-full size-24 bg-linear-to-br from-primary to-secondary text-primary-content text-3xl font-bold">
									{initial()}
								</div>
							}
						>
							<div class="avatar">
								<div class="rounded-full size-24 ring ring-primary ring-offset-2 ring-offset-base-100">
									<img src={displayAvatar()} alt="头像预览" />
								</div>
							</div>
						</Show>
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
						onChange={handleAvatarUpload}
					/>
				</Match>
			</Switch>

			{/* 预设头像（仅编辑态） */}
			<Show when={mode() === "edit"}>
				<PresetAvatars selected={avatar()} onSelect={handlePresetSelect} />
			</Show>

			{/* 分割线 */}
			<div class="divider w-full" />

			{/* 字段区域 */}
			<fieldset class="fieldset w-full">
				<legend class="fieldset-legend text-[14px]">用户名</legend>
				<input
					type="text"
					class="input w-full"
					value={user()?.name || ""}
					disabled
				/>
			</fieldset>

			<fieldset class="fieldset w-full">
				<legend class="fieldset-legend text-[14px]">昵称</legend>
				<Switch>
					<Match when={mode() === "view"}>
						<div class="input w-full bg-base-200 cursor-default">{displayName()}</div>
					</Match>
					<Match when={mode() === "edit"}>
						<input
							type="text"
							class="input w-full"
							value={displayName()}
							placeholder="请输入昵称"
							onInput={(e) => setDisplayName(e.target.value)}
						/>
					</Match>
				</Switch>
			</fieldset>

			<fieldset class="fieldset w-full">
				<legend class="fieldset-legend text-[14px]">角色</legend>
				<input
					type="text"
					class="input w-full"
					value={user()?.role || ""}
					disabled
				/>
			</fieldset>

			{/* 操作按钮 */}
			<Switch>
				<Match when={mode() === "view"}>
					<button
						type="button"
						class="btn btn-primary w-full"
						onClick={enterEdit}
					>
						编辑资料
					</button>
				</Match>
				<Match when={mode() === "edit"}>
					<div class="flex gap-3 w-full">
						<button
							type="button"
							class="btn btn-ghost flex-1"
							onClick={cancelEdit}
						>
							取消
						</button>
						<button
							type="button"
							class="btn btn-primary flex-1"
							onClick={handleSave}
							disabled={saving()}
						>
							<Show when={!saving()} fallback={<span class="loading loading-spinner loading-sm" />}>
								保存
							</Show>
						</button>
					</div>
				</Match>
			</Switch>
		</div>
	);
};

export default ProfileForm;
