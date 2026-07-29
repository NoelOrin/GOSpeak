import { type Component, createSignal } from "solid-js";
import { createGuild } from "@/api/guild";
import guildStore from "@/stores/guildStore";

const CreateGuildModal: Component<{
	ref: HTMLDialogElement | ((el: HTMLDialogElement) => void);
	onClose: () => void;
}> = (props) => {
	const [name, setName] = createSignal("");
	const [description, setDescription] = createSignal("");
	const [isPublic, setIsPublic] = createSignal(false);
	const [loading, setLoading] = createSignal(false);
	const [error, setError] = createSignal("");

	const handleCreate = async () => {
		if (!name().trim()) {
			setError("请输入服务器名称");
			return;
		}
		setLoading(true);
		setError("");
		try {
			const guild = await createGuild({
				name: name().trim(),
				description: description().trim() || undefined,
				is_public: isPublic(),
			});
			guildStore.addGuild(guild);
			guildStore.setCurrentGuild(guild.uuid);
			props.onClose();
		} catch (e: any) {
			setError(e?.response?.data?.msg || "创建失败");
		} finally {
			setLoading(false);
		}
	};

	return (
		<dialog ref={props.ref} class="modal" onClose={props.onClose}>
			<div class="modal-box">
				<h3 class="font-bold text-lg mb-4">创建语音服务器</h3>
				<div class="form-control mb-3">
					<label class="label" for="guild-create-name">
						<span class="label-text">服务器名称</span>
					</label>
					<input
						id="guild-create-name"
						type="text"
						class="input input-bordered"
						value={name()}
						onInput={(e) => setName(e.currentTarget.value)}
						placeholder="输入服务器名称"
					/>
				</div>
				<div class="form-control mb-3">
					<label class="label" for="guild-create-desc">
						<span class="label-text">描述（可选）</span>
					</label>
					<textarea
						id="guild-create-desc"
						class="textarea textarea-bordered"
						value={description()}
						onInput={(e) => setDescription(e.currentTarget.value)}
						placeholder="简要描述"
						rows={3}
					/>
				</div>
				<div class="form-control mb-4">
					<label class="cursor-pointer label justify-start gap-2">
						<input
							type="checkbox"
							class="checkbox"
							checked={isPublic()}
							onChange={(e) => setIsPublic(e.currentTarget.checked)}
						/>
						<span class="label-text">公开服务器</span>
					</label>
				</div>
				{error() && (
					<div class="alert alert-error mb-3">
						<span>{error()}</span>
					</div>
				)}
				<div class="modal-action">
					<button class="btn" onClick={props.onClose}>
						取消
					</button>
					<button
						class="btn btn-primary"
						onClick={handleCreate}
						disabled={loading()}
					>
						{loading() ? "创建中..." : "创建"}
					</button>
				</div>
			</div>
		</dialog>
	);
};

export default CreateGuildModal;
