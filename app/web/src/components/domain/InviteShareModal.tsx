import {
	Button,
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from "@gospeak/components";
import { inDesktop } from "@gospeak/desktop";
import {
	domainDeepLinkUrl,
	extractDomainInviteCode,
} from "@/utils/domainInvite";
import QRCode from "qrcode";
import { useEffect, useRef, useState } from "react";

export default function InviteShareModal({
	open,
	inviteUrl,
	onClose,
}: {
	open: boolean;
	inviteUrl: string;
	onClose: () => void;
}) {
	const [qrDataUrl, setQrDataUrl] = useState("");
	const [qrError, setQrError] = useState("");
	const [copied, setCopied] = useState(false);
	const [copyError, setCopyError] = useState("");
	const [copiedDeepLink, setCopiedDeepLink] = useState(false);
	const copyTimer = useRef<ReturnType<typeof setTimeout> | undefined>(
		undefined,
	);
	const deepLinkTimer = useRef<ReturnType<typeof setTimeout> | undefined>(
		undefined,
	);

	const code = extractDomainInviteCode(inviteUrl);
	const deepLink = code ? domainDeepLinkUrl(code) : "";

	useEffect(() => {
		let version = 0;
		const currentVersion = ++version;
		if (!inviteUrl) {
			setQrDataUrl("");
			setQrError("");
			return;
		}
		void QRCode.toDataURL(inviteUrl, { width: 240, margin: 2 })
			.then((dataUrl) => {
				if (currentVersion !== version) return;
				setQrDataUrl(dataUrl);
				setQrError("");
			})
			.catch(() => {
				setQrDataUrl("");
				setQrError("二维码生成失败，请使用复制链接分享");
			});
	}, [inviteUrl]);

	useEffect(
		() => () => {
			if (copyTimer.current !== undefined) clearTimeout(copyTimer.current);
			if (deepLinkTimer.current !== undefined)
				clearTimeout(deepLinkTimer.current);
		},
		[],
	);

	const copyLink = async () => {
		setCopyError("");
		try {
			await navigator.clipboard.writeText(inviteUrl);
			setCopied(true);
			if (copyTimer.current !== undefined) clearTimeout(copyTimer.current);
			copyTimer.current = setTimeout(() => setCopied(false), 2000);
		} catch {
			setCopyError("复制失败，请手动复制上方链接");
		}
	};

	const copyDeepLink = async () => {
		setCopyError("");
		try {
			await navigator.clipboard.writeText(deepLink);
			setCopiedDeepLink(true);
			if (deepLinkTimer.current !== undefined)
				clearTimeout(deepLinkTimer.current);
			deepLinkTimer.current = setTimeout(() => setCopiedDeepLink(false), 2000);
		} catch {
			setCopyError("复制失败，请手动复制上方链接");
		}
	};

	if (!open) return null;

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>分享邀请</DialogTitle>
				</DialogHeader>
				<div className="flex flex-col items-center gap-4">
					{qrDataUrl && (
						<img
							src={qrDataUrl}
							alt="邀请二维码"
							className="h-60 w-60 rounded-lg bg-white object-contain p-2"
						/>
					)}
					{qrError && (
						<p className="text-destructive w-full text-center text-xs">
							{qrError}
						</p>
					)}
					<div className="bg-muted text-muted-foreground w-full rounded-lg px-3 py-2 text-sm break-all">
						{inviteUrl}
					</div>
					{copyError && (
						<p className="text-destructive w-full text-center text-xs">
							{copyError}
						</p>
					)}
					<Button className="w-full" onClick={copyLink}>
						{copied ? "已复制" : "复制邀请链接"}
					</Button>
					{inDesktop() && deepLink && (
						<div className="flex w-full flex-col gap-2">
							<div className="text-faint-foreground w-full rounded-lg border border-dashed px-3 py-2 text-xs break-all">
								{deepLink}
							</div>
							<Button
								variant="outline"
								className="w-full"
								onClick={copyDeepLink}
							>
								{copiedDeepLink ? "已复制" : "复制桌面端深链"}
							</Button>
							<p className="text-faint-foreground text-center text-xs">
								已装 GOSpeak 桌面端的人打开深链会直接跳到这个邀请。
							</p>
						</div>
					)}
				</div>
				<div className="flex justify-end">
					<Button variant="outline" onClick={onClose}>
						关闭
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
