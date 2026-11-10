import { Dialog, DialogContent, DialogTitle } from "@gospeak/components";
import { Bell, X } from "lucide-react";
import { useEffect, useState } from "react";
import MarkdownText from "@/components/textRoom/MarkdownText";
import type { AnnouncementDTO } from "@/protocol/announcement";
import {
	notificationStore,
	useNotificationStore,
} from "@/stores/notificationStore";

const LEVEL_STYLES: Record<AnnouncementDTO["level"], string> = {
	info: "text-info",
	warning: "text-warning",
	critical: "text-destructive",
};

const SCOPE_LABELS: Record<AnnouncementDTO["scope_type"], string> = {
	platform: "全站",
	domain: "域内",
};

function formatTime(ms: number): string {
	const date = new Date(ms);
	return `${date.getMonth() + 1}/${date.getDate()} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

interface NotificationModalProps {
	open: boolean;
	onClose: () => void;
}

export default function NotificationModal({
	open,
	onClose,
}: NotificationModalProps) {
	const items = useNotificationStore((s) => s.items);
	const loading = useNotificationStore((s) => s.loading);
	const [selected, setSelected] = useState<string | null>(null);

	useEffect(() => {
		if (!open) return;
		void notificationStore.loadList();
	}, [open]);

	if (!open) return null;

	const current = items.find((item) => item.uuid === selected) ?? items[0];

	const select = (item: AnnouncementDTO) => {
		setSelected(item.uuid);
		void notificationStore.markRead(item.uuid);
	};

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent
				showClose={false}
				className="flex h-[min(80vh,640px)] max-w-4xl flex-col gap-0 overflow-hidden p-0"
			>
				<div className="border-border flex items-center justify-between border-b px-4 py-3">
					<div className="flex items-center gap-2">
						<Bell size={18} />
						<DialogTitle className="text-base font-semibold">通知</DialogTitle>
					</div>
					<button
						type="button"
						onClick={onClose}
						className="hover:bg-muted flex size-7 items-center justify-center rounded-sm"
						aria-label="关闭"
					>
						<X size={16} />
					</button>
				</div>

				<div className="flex min-h-0 flex-1">
					<div className="border-border w-72 shrink-0 overflow-y-auto border-r">
						{loading && items.length === 0 ? (
							<div className="text-faint-foreground p-4 text-sm">加载中...</div>
						) : items.length === 0 ? (
							<div className="text-faint-foreground p-8 text-center text-sm">
								暂无通知
							</div>
						) : (
							items.map((item) => (
								<button
									key={item.uuid}
									type="button"
									onClick={() => select(item)}
									className={`border-border hover:bg-muted flex w-full flex-col items-start gap-1 border-b px-3 py-2.5 text-left ${
										current?.uuid === item.uuid ? "bg-muted" : ""
									}`}
								>
									<div className="flex w-full items-center gap-1.5">
										{!item.is_read && (
											<span className="bg-primary size-1.5 shrink-0 rounded-full" />
										)}
										<span className="truncate text-sm font-medium">
											{item.title}
										</span>
									</div>
									<div className="text-faint-foreground flex items-center gap-2 text-2xs">
										<span className={LEVEL_STYLES[item.level]}>
											{SCOPE_LABELS[item.scope_type]}
										</span>
										<span>
											{formatTime(item.published_at ?? item.created_at)}
										</span>
									</div>
								</button>
							))
						)}
					</div>

					<div className="min-w-0 flex-1 overflow-y-auto p-5">
						{current ? (
							<>
								<h2 className="mb-1 text-lg font-semibold">{current.title}</h2>
								<div className="text-faint-foreground mb-4 text-xs">
									{current.created_by_name} ·{" "}
									{formatTime(current.published_at ?? current.created_at)}
								</div>
								<MarkdownText text={current.content} />
							</>
						) : (
							<div className="text-faint-foreground flex h-full items-center justify-center text-sm">
								选择一条通知查看详情
							</div>
						)}
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}
