import type { Component, JSX } from "solid-js";
import { Show } from "solid-js";

export interface ConfirmModalProps {
	open: boolean;
	title?: string;
	message: JSX.Element | string;
	confirmText?: string;
	cancelText?: string;
	confirmClass?: string;
	loading?: boolean;
	dialogRef: (el: HTMLDialogElement) => void;
	onClose: () => void;
	onConfirm: () => void | Promise<void>;
}

const ConfirmModal: Component<ConfirmModalProps> = (props) => {
	const requestClose = () => {
		if (props.loading) return;
		props.onClose();
	};

	return (
		<Show when={props.open}>
			<dialog
				ref={(el) => {
					props.dialogRef(el);
				}}
				class="modal"
				onCancel={(e) => {
					if (props.loading) {
						e.preventDefault();
						return;
					}
				}}
				onClose={requestClose}
			>
				<div class="modal-box max-w-md">
					<h3 class="mb-3 font-bold text-lg">{props.title ?? "请确认"}</h3>
					<div class="text-base-content/80 text-sm leading-6">
						{props.message}
					</div>
					<div class="modal-action">
						<button
							type="button"
							class="btn"
							disabled={props.loading}
							onClick={requestClose}
						>
							{props.cancelText ?? "取消"}
						</button>
						<button
							type="button"
							class={props.confirmClass ?? "btn btn-error"}
							disabled={props.loading}
							onClick={() => {
								void props.onConfirm();
							}}
						>
							<Show when={props.loading}>
								<span class="loading loading-spinner loading-xs" />
							</Show>
							{props.confirmText ?? "确认"}
						</button>
					</div>
				</div>
				<form method="dialog" class="modal-backdrop">
					<button
						type="submit"
						aria-label="关闭"
						disabled={props.loading}
						onClick={(e) => {
							if (props.loading) e.preventDefault();
						}}
					/>
				</form>
			</dialog>
		</Show>
	);
};

export default ConfirmModal;
