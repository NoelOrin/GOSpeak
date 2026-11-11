import {
	Button,
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
	Input,
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
	toast,
} from "@gospeak/components";
import { useState } from "react";

const EXPIRY_OPTIONS = [
	{ label: "永久", hours: 0 },
	{ label: "1 小时", hours: 1 },
	{ label: "24 小时", hours: 24 },
	{ label: "7 天", hours: 24 * 7 },
	{ label: "30 天", hours: 24 * 30 },
] as const;

export default function InviteCodeCreateModal({
	open,
	onClose,
	onCreate,
}: {
	open: boolean;
	onClose: () => void;
	onCreate: (params: {
		label: string;
		expiresInHours: number;
		maxUses: number;
	}) => Promise<void>;
}) {
	const [label, setLabel] = useState("");
	const [expiryHours, setExpiryHours] = useState("0");
	const [maxUses, setMaxUses] = useState("0");
	const [busy, setBusy] = useState(false);

	async function submit() {
		if (busy) return;
		const parsedMaxUses = Number(maxUses || "0");
		if (
			!Number.isInteger(parsedMaxUses) ||
			parsedMaxUses < 0 ||
			parsedMaxUses > 65535
		) {
			toast.error("使用上限必须在 0 到 65535 之间");
			return;
		}
		setBusy(true);
		try {
			await onCreate({
				label: label.trim(),
				expiresInHours: Number(expiryHours),
				maxUses: parsedMaxUses,
			});
			setLabel("");
			setExpiryHours("0");
			setMaxUses("0");
			onClose();
		} finally {
			setBusy(false);
		}
	}

	return (
		<Dialog open={open} onOpenChange={(next) => !next && onClose()}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>新建邀请码</DialogTitle>
				</DialogHeader>
				<div className="flex flex-col gap-4">
					<label className="flex flex-col gap-1 text-sm">
						备注名
						<Input
							value={label}
							onChange={(e) => setLabel(e.target.value)}
							placeholder="例如：KOL 推广"
						/>
					</label>
					<div className="flex flex-col gap-1 text-sm">
						有效期
						<Select value={expiryHours} onValueChange={setExpiryHours}>
							<SelectTrigger>
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{EXPIRY_OPTIONS.map((opt) => (
									<SelectItem key={opt.hours} value={String(opt.hours)}>
										{opt.label}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<label className="flex flex-col gap-1 text-sm">
						最大使用次数（0 表示不限，上限 65535）
						<Input
							type="number"
							min={0}
							max={65535}
							value={maxUses}
							onChange={(e) => setMaxUses(e.target.value)}
						/>
					</label>
				</div>
				<div className="flex justify-end gap-2 pt-2">
					<Button variant="outline" onClick={onClose} disabled={busy}>
						取消
					</Button>
					<Button onClick={() => void submit()} disabled={busy}>
						{busy ? "创建中…" : "创建"}
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
