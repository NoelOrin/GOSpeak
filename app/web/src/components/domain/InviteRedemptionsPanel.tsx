import { Button } from "@gospeak/components";
import { useEffect, useState } from "react";
import {
	type DomainInvite,
	type InviteRedemption,
	listRedemptions,
} from "@/api/invite";
import { toastActionError } from "@/utils/toast";

const FAIL_LABEL: Record<string, string> = {
	expired: "已过期",
	exhausted: "已达上限",
	invalid: "无效",
	disabled: "已禁用",
	rejected: "审核拒绝",
	already_member: "已是成员",
};

const PAGE_SIZE = 20;

export default function InviteRedemptionsPanel({
	domainUUID,
	invite,
	onClose,
}: {
	domainUUID: string;
	invite: DomainInvite;
	onClose: () => void;
}) {
	const [items, setItems] = useState<InviteRedemption[]>([]);
	const [total, setTotal] = useState(0);
	const [page, setPage] = useState(1);
	const [loading, setLoading] = useState(false);

	useEffect(() => {
		let cancelled = false;
		setLoading(true);
		void listRedemptions(domainUUID, invite.id, page, PAGE_SIZE)
			.then((result) => {
				if (cancelled) return;
				setItems(result.items);
				setTotal(result.total);
			})
			.catch((error) => {
				if (!cancelled) toastActionError(error, "加载使用记录失败");
			})
			.finally(() => {
				if (!cancelled) setLoading(false);
			});
		return () => {
			cancelled = true;
		};
	}, [domainUUID, invite.id, page]);

	const pageCount = Math.max(1, Math.ceil(total / PAGE_SIZE));

	return (
		<div className="border-border bg-card flex h-full w-80 flex-col gap-3 border-l p-4">
			<div className="flex items-center justify-between">
				<h3 className="text-sm font-semibold">使用记录</h3>
				<Button variant="ghost" size="sm" onClick={onClose}>
					关闭
				</Button>
			</div>
			<p className="text-muted-foreground text-xs">
				{invite.label || invite.code.slice(0, 8)} · 共 {total} 条
			</p>
			{loading && <p className="text-muted-foreground text-xs">加载中…</p>}
			{!loading && items.length === 0 && (
				<p className="text-muted-foreground text-xs">还没有使用记录</p>
			)}
			<ul className="flex-1 overflow-auto">
				{items.map((item) => (
					<li
						key={item.id}
						className="border-border/60 border-b py-2 text-xs last:border-0"
					>
						<div className="flex items-center justify-between">
							<span>
								{item.success
									? "成功"
									: `失败 · ${FAIL_LABEL[item.fail_reason ?? ""] ?? item.fail_reason}`}
							</span>
							<span className="text-faint-foreground">
								{new Date(item.created_at).toLocaleString("zh-CN")}
							</span>
						</div>
						{item.user_uuid && (
							<p className="text-faint-foreground mt-1 truncate">
								{item.user_uuid}
							</p>
						)}
					</li>
				))}
			</ul>
			{pageCount > 1 && (
				<div className="flex items-center justify-between">
					<Button
						variant="outline"
						size="sm"
						disabled={page <= 1}
						onClick={() => setPage((p) => p - 1)}
					>
						上一页
					</Button>
					<span className="text-muted-foreground text-xs">
						{page} / {pageCount}
					</span>
					<Button
						variant="outline"
						size="sm"
						disabled={page >= pageCount}
						onClick={() => setPage((p) => p + 1)}
					>
						下一页
					</Button>
				</div>
			)}
		</div>
	);
}
