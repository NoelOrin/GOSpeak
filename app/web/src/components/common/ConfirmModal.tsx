import {
	Button,
	Dialog,
	DialogContent,
	DialogTitle,
	Input,
} from "@gospeak/components";
import { Loader2 } from "lucide-react";
import { type ReactNode, useEffect, useId, useState } from "react";

export interface ConfirmModalProps {
	open: boolean;
	title?: string;
	message: ReactNode | string;
	confirmText?: string;
	cancelText?: string;
	confirmVariant?: "default" | "destructive";
	loading?: boolean;
	/**
	 * 高危操作的二次确认短语：填入该字段后确认按钮才可点。
	 * 传入时建议用被操作对象的名字（域名称 / 用户名称），让用户"读一遍再删"。
	 */
	confirmPhrase?: string;
	onClose: () => void;
	onConfirm: () => void | Promise<void>;
}

export default function ConfirmModal({
	open,
	title,
	message,
	confirmText,
	cancelText,
	confirmVariant = "destructive",
	loading,
	confirmPhrase,
	onClose,
	onConfirm,
}: ConfirmModalProps) {
	const [phrase, setPhrase] = useState("");
	const inputId = useId();

	// 每次重新打开都要清空，避免上一次输入残留导致误判"已确认"
	useEffect(() => {
		if (open) setPhrase("");
	}, [open]);

	const phraseMatched = !confirmPhrase || phrase.trim() === confirmPhrase;

	const requestClose = () => {
		if (loading) return;
		onClose();
	};

	return (
		<Dialog open={open} onOpenChange={(v) => !v && requestClose()}>
			<DialogContent showClose={false} className="max-w-md">
				<DialogTitle className="mb-3 text-lg font-bold">
					{title ?? "请确认"}
				</DialogTitle>
				<div className="text-foreground text-sm leading-6">{message}</div>

				{confirmPhrase && (
					<div className="mt-3 flex flex-col gap-1.5">
						<label htmlFor={inputId} className="text-muted-foreground text-xs">
							请输入「{confirmPhrase}」以确认
						</label>
						<Input
							id={inputId}
							value={phrase}
							autoComplete="off"
							disabled={loading}
							onChange={(e) => setPhrase(e.target.value)}
							onKeyDown={(e) => {
								if (e.key === "Enter" && phraseMatched && !loading) {
									void onConfirm();
								}
							}}
						/>
					</div>
				)}

				<div className="mt-4 flex justify-end gap-2">
					<Button variant="outline" disabled={loading} onClick={requestClose}>
						{cancelText ?? "取消"}
					</Button>
					<Button
						variant={confirmVariant}
						disabled={loading || !phraseMatched}
						onClick={() => void onConfirm()}
					>
						{loading && <Loader2 className="size-4 animate-spin" />}
						{confirmText ?? "确认"}
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
