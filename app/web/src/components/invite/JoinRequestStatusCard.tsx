import { Button } from "@gospeak/components";
import { Clock, XCircle } from "lucide-react";

export interface JoinRequestStatusCardProps {
	domainName: string;
	iconUrl: string;
	status: "pending" | "rejected";
	submittedAt: string;
	cancelling: boolean;
	onCancel: () => void;
	onReapply: () => void;
}

export function JoinRequestStatusCard(props: JoinRequestStatusCardProps) {
	const pending = props.status === "pending";
	return (
		<div className="flex flex-col gap-4">
			<div className="flex items-center gap-3">
				{props.iconUrl ? (
					<img
						src={props.iconUrl}
						alt={props.domainName}
						className="size-14 shrink-0 rounded-xl object-cover"
					/>
				) : (
					<div className="bg-primary text-primary-foreground flex size-14 shrink-0 items-center justify-center rounded-xl text-xl font-bold">
						{(Array.from(props.domainName.trim())[0] ?? "?").toUpperCase()}
					</div>
				)}
				<div className="min-w-0">
					<h1 className="truncate text-lg font-semibold">{props.domainName}</h1>
					<p className="text-muted-foreground text-xs">
						提交于 {props.submittedAt}
					</p>
				</div>
			</div>

			<div className="border-border flex items-start gap-2 rounded-lg border px-3 py-2.5 text-sm">
				{pending ? (
					<Clock size={16} className="text-warning mt-0.5 shrink-0" />
				) : (
					<XCircle size={16} className="text-destructive mt-0.5 shrink-0" />
				)}
				<div className="min-w-0">
					<p className="font-medium">{pending ? "申请审核中" : "申请未通过"}</p>
					<p className="text-muted-foreground mt-1 text-xs leading-relaxed">
						{pending
							? "管理员通过后即可进入该域。审核期间你暂时无法进入。"
							: "你可以重新提交申请，或联系域管理员了解原因。"}
					</p>
				</div>
			</div>

			<div className="flex gap-2">
				{pending ? (
					<Button
						variant="outline"
						className="flex-1"
						disabled={props.cancelling}
						onClick={props.onCancel}
					>
						{props.cancelling ? "撤销中..." : "撤销申请"}
					</Button>
				) : (
					<Button className="flex-1" onClick={props.onReapply}>
						重新申请
					</Button>
				)}
			</div>
		</div>
	);
}
