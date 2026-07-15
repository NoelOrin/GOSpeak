import { Show } from "solid-js";
import PasswordChangeForm from "@/components/form/PasswordChangeForm";

export interface ForcePasswordChangeModalProps {
	open: boolean;
	dialogRef: (el: HTMLDialogElement) => void;
	onClose: () => void;
	onSubmit: (values: {
		username?: string;
		oldPassword?: string;
		newPassword: string;
		name?: string;
	}) => Promise<void>;
}

export default function ForcePasswordChangeModal(
	props: ForcePasswordChangeModalProps,
) {
	return (
		<Show when={props.open}>
			<dialog
				ref={(el) => {
					props.dialogRef(el);
				}}
				class="modal"
				onClose={props.onClose}
			>
				<div class="modal-box">
					<h3 class="font-bold text-lg mb-2">修改密码</h3>
					<p class="text-sm text-base-content/60 mb-4">
						检测到您使用的是默认密码，请修改密码后继续。
					</p>
					<PasswordChangeForm
						showOldPassword={false}
						showName={true}
						submitText="修改密码"
						onSubmit={props.onSubmit}
					/>
				</div>
				<form method="dialog" class="modal-backdrop">
					<button>close</button>
				</form>
			</dialog>
		</Show>
	);
}
