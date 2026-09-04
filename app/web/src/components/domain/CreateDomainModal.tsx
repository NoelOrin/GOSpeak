import { type Component, createSignal } from "solid-js";
import { type Domain, createDomain } from "@/api/domain";
import domainStore from "@/stores/domainStore";

const CreateDomainModal: Component<{
	ref: HTMLDialogElement | ((el: HTMLDialogElement) => void);
	onClose: () => void;
	onCreated?: (domain: Domain) => void;
}> = (props) => {
	const [name, setName] = createSignal("");
	const [description, setDescription] = createSignal("");
	const [isPublic, setIsPublic] = createSignal(false);
	const [loading, setLoading] = createSignal(false);
	const [error, setError] = createSignal("");

	const handleCreate = async () => {
		if (!name().trim()) {
			setError("请输入域名称");
			return;
		}
		setLoading(true);
		setError("");
		try {
			const domain = await createDomain({
				name: name().trim(),
				description: description().trim() || undefined,
				is_public: isPublic(),
			});
			domainStore.addDomain(domain);
			domainStore.setCurrentDomain(domain.uuid);
			props.onClose();
			props.onCreated?.(domain);
		} catch (e: any) {
			setError(e?.response?.data?.msg || "创建失败");
		} finally {
			setLoading(false);
		}
	};

	return (
		<dialog ref={props.ref} class="modal" onClose={props.onClose}>
			<div class="modal-box">
				<h3 class="font-bold text-lg mb-4">创建域</h3>
				<div class="form-control mb-3">
					<label class="label" for="domain-create-name">
						<span class="label-text">域名称</span>
					</label>
					<input
						id="domain-create-name"
						type="text"
						class="input input-bordered"
						value={name()}
						onInput={(e) => setName(e.currentTarget.value)}
						placeholder="输入域名称"
					/>
				</div>
				<div class="form-control mb-3">
					<label class="label" for="domain-create-desc">
						<span class="label-text">描述（可选）</span>
					</label>
					<textarea
						id="domain-create-desc"
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
						<span class="label-text">公开域</span>
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

export default CreateDomainModal;
