import { Button } from "@gospeak/components";
import { Ban, Copy, History } from "lucide-react";
import { type DomainInvite, inviteStatus } from "@/api/invite";

const STATUS_LABEL: Record<ReturnType<typeof inviteStatus>, string> = {
	active: "生效中",
	expired: "已过期",
	exhausted: "已用完",
	disabled: "已禁用",
};

function usageText(invite: DomainInvite): string {
	const used = `${invite.used_count}`;
	return invite.max_uses > 0
		? `${used} / ${invite.max_uses}`
		: `${used} / 不限`;
}

function expiryText(invite: DomainInvite): string {
	if (!invite.expires_at) return "永久";
	return new Date(invite.expires_at).toLocaleString("zh-CN");
}

export default function InviteCodeTable({
	invites,
	onDisable,
	onShowRedemptions,
	onCopy,
}: {
	invites: DomainInvite[];
	onDisable: (inviteID: number) => void;
	onShowRedemptions: (invite: DomainInvite) => void;
	onCopy?: (invite: DomainInvite) => void;
}) {
	if (invites.length === 0) {
		return <p className="text-muted-foreground text-sm">还没有邀请码</p>;
	}
	return (
		<table className="w-full text-sm">
			<thead>
				<tr className="text-muted-foreground border-border border-b text-left text-xs">
					<th className="py-2">备注</th>
					<th className="py-2">邀请码</th>
					<th className="py-2">有效期</th>
					<th className="py-2">已用 / 上限</th>
					<th className="py-2">状态</th>
					<th className="py-2 text-right">操作</th>
				</tr>
			</thead>
			<tbody>
				{invites.map((invite) => {
					const status = inviteStatus(invite);
					return (
						<tr
							key={invite.id}
							className="border-border/60 border-b last:border-0"
						>
							<td className="py-2">{invite.label || "—"}</td>
							<td className="py-2">
								<code className="text-xs">{`${invite.code.slice(0, 8)}…${invite.code.slice(-4)}`}</code>
							</td>
							<td className="py-2 text-xs">{expiryText(invite)}</td>
							<td className="py-2 text-xs">{usageText(invite)}</td>
							<td className="py-2 text-xs">{STATUS_LABEL[status]}</td>
							<td className="py-2 text-right">
								<div className="inline-flex gap-1">
									{onCopy && (
										<Button
											variant="ghost"
											size="sm"
											onClick={() => onCopy(invite)}
										>
											<Copy size={14} />
										</Button>
									)}
									<Button
										variant="ghost"
										size="sm"
										onClick={() => onShowRedemptions(invite)}
									>
										<History size={14} />
									</Button>
									{status !== "disabled" && (
										<Button
											variant="ghost"
											size="sm"
											onClick={() => onDisable(invite.id)}
										>
											<Ban size={14} />
											禁用
										</Button>
									)}
								</div>
							</td>
						</tr>
					);
				})}
			</tbody>
		</table>
	);
}
