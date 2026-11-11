import { Button } from "@gospeak/components";
import { Loader2, Search } from "lucide-react";
import type { JoinRequestListItem } from "@/api/inviteJoin";

export type JoinRequestStatusFilter = "pending" | "approved" | "rejected";

const STATUS_LABELS: Record<JoinRequestStatusFilter, string> = {
	pending: "待审核",
	approved: "已通过",
	rejected: "已拒绝",
};

export interface JoinRequestTableProps {
	items: JoinRequestListItem[];
	total: number;
	counts: Record<string, number>;
	status: JoinRequestStatusFilter;
	keyword: string;
	page: number;
	pageSize: number;
	loading: boolean;
	error: string;
	reviewing: number;
	onStatusChange: (status: JoinRequestStatusFilter) => void;
	onKeywordChange: (keyword: string) => void;
	onSearch: () => void;
	onPageChange: (page: number) => void;
	onReview: (requestID: number, approve: boolean) => void;
}

function formatTime(raw: string): string {
	const date = new Date(raw);
	if (Number.isNaN(date.getTime())) return "";
	return date.toLocaleString("zh-CN", { hour12: false });
}

export function JoinRequestTable(props: JoinRequestTableProps) {
	const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize));

	return (
		<div className="flex min-w-0 flex-col gap-3">
			<div className="flex flex-wrap items-center gap-2">
				<div className="border-border inline-flex overflow-hidden rounded-sm border">
					{(["pending", "approved", "rejected"] as const).map((status) => (
						<button
							key={status}
							type="button"
							className={`px-3 py-1.5 text-xs transition-colors ${
								props.status === status
									? "bg-primary text-primary-foreground"
									: "text-muted-foreground hover:bg-muted"
							}`}
							onClick={() => props.onStatusChange(status)}
						>
							{STATUS_LABELS[status]}（{props.counts[status] ?? 0}）
						</button>
					))}
				</div>
				<div className="flex min-w-0 flex-1 items-center gap-2">
					<input
						className="border-input bg-card text-foreground focus-visible:ring-ring flex h-8 min-w-0 flex-1 rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2"
						placeholder="搜索申请人"
						value={props.keyword}
						onChange={(e) => props.onKeywordChange(e.currentTarget.value)}
						onKeyDown={(e) => {
							if (e.key === "Enter") props.onSearch();
						}}
					/>
					<Button
						variant="outline"
						size="sm"
						className="gap-1.5"
						onClick={props.onSearch}
					>
						<Search size={14} />
						搜索
					</Button>
				</div>
			</div>

			{props.error && (
				<p
					role="alert"
					className="border-destructive/50 bg-destructive/10 text-destructive rounded-sm border px-3 py-2 text-sm"
				>
					{props.error}
				</p>
			)}

			{props.loading ? (
				<div className="text-faint-foreground flex items-center justify-center py-8">
					<Loader2 className="size-5 animate-spin" />
				</div>
			) : props.items.length === 0 ? (
				<div className="text-faint-foreground flex items-center justify-center py-8 text-sm">
					暂无加入申请
				</div>
			) : (
				<div className="min-w-0 overflow-x-auto">
					<table className="w-full min-w-[520px] border-collapse text-sm">
						<thead>
							<tr className="border-border bg-muted/50 text-muted-foreground border-b text-xs font-semibold">
								<th className="px-3 py-2 text-left">申请人</th>
								<th className="px-3 py-2 text-left">提交时间</th>
								<th className="px-3 py-2 text-left">状态</th>
								<th className="px-3 py-2 text-right">操作</th>
							</tr>
						</thead>
						<tbody>
							{props.items.map((item) => {
								const pending = item.status === "pending";
								return (
									<tr
										key={item.id}
										className="border-border hover:bg-muted/40 border-b"
									>
										<td className="px-3 py-2">
											<div className="font-medium">
												{item.display_name || item.name}
											</div>
											<div className="text-faint-foreground text-xs">
												@{item.name}
											</div>
										</td>
										<td className="text-muted-foreground px-3 py-2 text-xs">
											{formatTime(item.created_at)}
										</td>
										<td className="px-3 py-2 text-xs">
											{item.status === "pending"
												? "待审核"
												: item.status === "approved"
													? "已通过"
													: "已拒绝"}
										</td>
										<td className="px-3 py-2 text-right">
											{pending && (
												<div className="inline-flex gap-1">
													<Button
														variant="outline"
														size="xs"
														disabled={props.reviewing === item.id}
														onClick={() => props.onReview(item.id, true)}
													>
														通过
													</Button>
													<Button
														variant="outline"
														size="xs"
														disabled={props.reviewing === item.id}
														onClick={() => props.onReview(item.id, false)}
													>
														拒绝
													</Button>
												</div>
											)}
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				</div>
			)}

			{props.total > props.pageSize && (
				<div className="flex items-center justify-between text-xs">
					<span className="text-faint-foreground">
						第 {props.page} / {totalPages} 页，共 {props.total} 条
					</span>
					<div className="flex gap-2">
						<Button
							variant="outline"
							size="xs"
							disabled={props.page <= 1}
							onClick={() => props.onPageChange(props.page - 1)}
						>
							上一页
						</Button>
						<Button
							variant="outline"
							size="xs"
							disabled={props.page >= totalPages}
							onClick={() => props.onPageChange(props.page + 1)}
						>
							下一页
						</Button>
					</div>
				</div>
			)}
		</div>
	);
}
