import { type Component, createSignal, onMount, Show } from "solid-js";
import QRCode from "qrcode";

const InviteShareModal: Component<{
	ref: HTMLDialogElement | ((el: HTMLDialogElement) => void);
	inviteUrl: string;
	onClose: () => void;
}> = (props) => {
	const [qrDataUrl, setQrDataUrl] = createSignal("");
	const [copied, setCopied] = createSignal(false);

	onMount(() => {
		void QRCode.toDataURL(props.inviteUrl, { width: 240, margin: 2 })
			.then(setQrDataUrl)
			.catch(() => setQrDataUrl(""));
	});

	const copyLink = async () => {
		await navigator.clipboard.writeText(props.inviteUrl);
		setCopied(true);
		setTimeout(() => setCopied(false), 2000);
	};

	return (
		<dialog ref={props.ref} class="modal" onClose={props.onClose}>
			<div class="modal-box">
				<h3 class="font-bold text-lg mb-4">分享邀请</h3>
				<div class="flex flex-col items-center gap-4">
					<Show when={qrDataUrl()}>
						<img
							src={qrDataUrl()}
							alt="邀请二维码"
							class="w-60 h-60 object-contain bg-white p-2 rounded-lg"
						/>
					</Show>
					<div class="w-full text-sm break-all text-base-content/70 bg-base-200 rounded-lg px-3 py-2">
						{props.inviteUrl}
					</div>
					<button
						type="button"
						class="btn btn-primary btn-sm w-full"
						onClick={copyLink}
					>
						{copied() ? "已复制" : "复制邀请链接"}
					</button>
				</div>
				<div class="modal-action">
					<button class="btn" onClick={props.onClose}>
						关闭
					</button>
				</div>
			</div>
		</dialog>
	);
};

export default InviteShareModal;
