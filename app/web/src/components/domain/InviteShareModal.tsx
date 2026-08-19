import {
	type Component,
	createSignal,
	createEffect,
	onCleanup,
	Show,
} from "solid-js";
import QRCode from "qrcode";

const InviteShareModal: Component<{
	ref: HTMLDialogElement | ((el: HTMLDialogElement) => void);
	inviteUrl: string;
	onClose: () => void;
}> = (props) => {
	const [qrDataUrl, setQrDataUrl] = createSignal("");
	const [qrError, setQrError] = createSignal("");
	const [copied, setCopied] = createSignal(false);
	const [copyError, setCopyError] = createSignal("");

	let qrVersion = 0;
	createEffect(() => {
		const url = props.inviteUrl;
		const version = ++qrVersion;
		if (!url) {
			setQrDataUrl("");
			setQrError("");
			return;
		}
		void QRCode.toDataURL(url, { width: 240, margin: 2 })
			.then((dataUrl) => {
				if (version !== qrVersion) return;
				setQrDataUrl(dataUrl);
				setQrError("");
			})
			.catch(() => {
				if (version !== qrVersion) return;
				setQrDataUrl("");
				setQrError("二维码生成失败，请使用复制链接分享");
			});
	});

	let copyTimer: ReturnType<typeof setTimeout> | undefined;
	onCleanup(() => {
		if (copyTimer !== undefined) clearTimeout(copyTimer);
	});
	const copyLink = async () => {
		setCopyError("");
		try {
			await navigator.clipboard.writeText(props.inviteUrl);
			setCopied(true);
			if (copyTimer !== undefined) clearTimeout(copyTimer);
			copyTimer = setTimeout(() => setCopied(false), 2000);
		} catch {
			setCopyError("复制失败，请手动复制上方链接");
		}
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
					<Show when={qrError()}>
						<p class="w-full text-center text-xs text-error">{qrError()}</p>
					</Show>
					<div class="w-full text-sm break-all text-base-content/70 bg-base-200 rounded-lg px-3 py-2">
						{props.inviteUrl}
					</div>
					<Show when={copyError()}>
						<p class="w-full text-center text-xs text-error">{copyError()}</p>
					</Show>
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
