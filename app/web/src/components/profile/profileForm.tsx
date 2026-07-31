import {
	createEffect,
	createSignal,
	type JSX,
	Match,
	Show,
	Switch,
} from "solid-js";
import { showToast } from "solid-notifications";
import { updateProfile } from "@/api/user";
import Avatar from "@/components/common/avatar";
import { useUpload } from "@/hooks/useUpload";
import userStore from "@/stores/userStore";
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
	footer?: JSX.Element;
};

const ProfileForm = (props: ProfileFormProps) => {
	const variant = () => props.variant ?? "page";
	const isCompact = () => variant() === "compact";
	const showUsername = () => props.showUsername ?? true;
	const showRole = () => props.showRole ?? true;

	const user = () => userStore.user();
	const [mode, setMode] = createSignal<ProfileFormMode>(
		props.initialMode ?? "view",
	);
	const [displayName, setDisplayName] = createSignal(
		user()?.display_name || user()?.name || "",
	);
	const [avatar, setAvatar] = createSignal(user()?.avatar || "");
	const [saving, setSaving] = createSignal(false);
	const [preview, setPreview] = createSignal<string | null>(null);
	const { upload, uploading } = useUpload("avatars");
	let fileInput!: HTMLInputElement;

	createEffect(() => {
		const u = user();
		if (u && mode() === "view") {
			setDisplayName(u.display_name || u.name || "");
			setAvatar(u.avatar || "");
			setPreview(null);
		}
	});

	const resetFromUser = () => {
		setDisplayName(user()?.display_name || user()?.name || "");
		setAvatar(user()?.avatar || "");
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
		const target = fileInput;
		const file = target.files?.[0];
		if (!file) return;

		if (file.size > 5 * 1024 * 1024) {
			showToast("头像文件不能超过 5MB", { type: "warning" });
			return;
		}
		if (
			!["image/jpeg", "image/png", "image/gif", "image/webp"].includes(
				file.type,
			)
		) {
			showToast("仅支持 JPG/PNG/GIF/WebP 格式", { type: "warning" });
			return;
		}

		const reader = new FileReader();
		reader.onload = (ev) => setPreview(ev.target?.result as string);
		reader.readAsDataURL(file);

		try {
			const result = await upload(file);
			setAvatar(result.public_url);
			showToast("头像上传成功", { type: "success" });
		} catch (err: any) {
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
			props.onSaved?.();
		} catch (err: any) {
		} finally {
			setSaving(false);
		}
	};

	const displayAvatar = () => preview() || avatar();
	const avatarName = () =>
		displayName() || user()?.display_name || user()?.name || "?";
	const avatarSize = () => (isCompact() ? "size-16" : "size-24");
	const avatarText = () => (isCompact() ? "text-xl" : "text-3xl");

	const AvatarBlock = () => (
		<Show
			when={displayAvatar()}
			fallback={
				<Avatar
					name={avatarName()}
					class={avatarSize()}
					textClass={avatarText()}
				/>
			}
		>
			<div class="avatar shrink-0">
				<div
					class="rounded-full ring ring-base-content/15 ring-offset-2 ring-offset-base-100"
					classList={{
						"size-16": isCompact(),
						"size-24": !isCompact(),
					}}
				>
					<img src={displayAvatar()!} alt="头像" />
				</div>
			</div>
		</Show>
	);

	return (
		<div
			class="flex w-full flex-col gap-5"
			classList={{
				"mx-auto max-w-lg items-center p-6": !isCompact(),
				"items-stretch": isCompact(),
			}}
		>
			{/* 头像 */}
			<div
				class="flex w-full gap-4"
				classList={{
					"flex-col items-center": !isCompact(),
					"items-center": isCompact(),
				}}
			>
				<Switch>
					<Match when={mode() === "view"}>
						<AvatarBlock />
					</Match>
					<Match when={mode() === "edit"}>
						<button
							type="button"
							class="relative group shrink-0 cursor-pointer"
							onClick={() => fileInput.click()}
							disabled={uploading()}
							title="点击上传头像"
						>
							<AvatarBlock />
							<div class="absolute inset-0 flex items-center justify-center rounded-full bg-black/45 opacity-0 transition-opacity group-hover:opacity-100">
								<Show
									when={!uploading()}
									fallback={
										<span class="loading loading-spinner loading-md text-white" />
									}
								>
									<span class="text-xs text-white sm:text-sm">更换头像</span>
								</Show>
							</div>
						</button>
						<input
							ref={fileInput}
							type="file"
							accept="image/jpeg,image/png,image/gif,image/webp"
							class="hidden"
							onChange={() => void handleAvatarUpload()}
						/>
					</Match>
				</Switch>

				<Show when={isCompact()}>
					<div class="min-w-0 flex-1">
						<div class="truncate text-base font-semibold">{avatarName()}</div>
						<div class="text-xs text-base-content/50">@{user()?.name}</div>
						<div class="mt-1 badge badge-ghost badge-sm">
							{user()?.role || "user"}
						</div>
					</div>
				</Show>
			</div>

			{/* 预设头像 */}
			<Show when={mode() === "edit"}>
				<div class="w-full">
					<PresetAvatars selected={avatar()} onSelect={handlePresetSelect} />
				</div>
			</Show>

			{/* 字段 */}
			<div class="flex w-full flex-col gap-3">
				<Show when={showUsername()}>
					<fieldset class="fieldset w-full">
						<legend class="fieldset-legend text-[14px]">用户名</legend>
						<input
							type="text"
							class="input w-full"
							value={user()?.name || ""}
							disabled
						/>
						<p class="mt-1 text-xs text-base-content/45">用户名不可修改</p>
					</fieldset>
				</Show>

				<fieldset class="fieldset w-full">
					<legend class="fieldset-legend text-[14px]">昵称</legend>
					<Switch>
						<Match when={mode() === "view"}>
							<div class="input w-full cursor-default bg-base-200">
								{displayName() || "—"}
							</div>
						</Match>
						<Match when={mode() === "edit"}>
							<input
								type="text"
								class="input w-full"
								value={displayName()}
								placeholder="请输入昵称"
								maxLength={32}
								onInput={(e) => setDisplayName(e.currentTarget.value)}
							/>
						</Match>
					</Switch>
				</fieldset>

				<Show when={showRole() && !isCompact()}>
					<fieldset class="fieldset w-full">
						<legend class="fieldset-legend text-[14px]">角色</legend>
						<input
							type="text"
							class="input w-full"
							value={user()?.role || ""}
							disabled
						/>
					</fieldset>
				</Show>
			</div>

			{/* 操作 */}
			<div class="flex w-full flex-wrap gap-2">
				<Switch>
					<Match when={mode() === "view"}>
						<button
							type="button"
							class="btn btn-outline btn-sm"
							classList={{ "w-full btn-md": !isCompact() }}
							onClick={enterEdit}
						>
							编辑资料
						</button>
					</Match>
					<Match when={mode() === "edit"}>
						<button
							type="button"
							class="btn btn-ghost btn-sm"
							classList={{ "flex-1 btn-md": !isCompact() }}
							onClick={cancelEdit}
							disabled={saving()}
						>
							取消
						</button>
						<button
							type="button"
							class="btn btn-outline btn-sm"
							classList={{ "flex-1 btn-md": !isCompact() }}
							onClick={() => void handleSave()}
							disabled={saving() || uploading()}
						>
							<Show
								when={!saving()}
								fallback={<span class="loading loading-spinner loading-sm" />}
							>
								保存
							</Show>
						</button>
					</Match>
				</Switch>
			</div>

			<Show when={props.footer}>
				<div class="w-full">{props.footer}</div>
			</Show>
		</div>
	);
};

export default ProfileForm;
