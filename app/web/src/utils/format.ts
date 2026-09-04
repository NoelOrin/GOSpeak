/**
 * 时间/时长格式化工具函数。
 */

/**
 * 计算距离过期还剩多久，返回人类可读格式。
 * @param expiresAt ISO 日期字符串，或 null
 * @returns "3h 25m" / "5m 30s" / "已过期" / "—"
 */
export function formatRemaining(expiresAt: string | null): string {
	if (!expiresAt) return "—";
	const remaining = new Date(expiresAt).getTime() - Date.now();
	if (remaining <= 0) return "已过期";
	const mins = Math.floor(remaining / 60000);
	const secs = Math.floor((remaining % 60000) / 1000);
	if (mins >= 60) {
		const hours = Math.floor(mins / 60);
		return `${hours}h ${mins % 60}m`;
	}
	return `${mins}m ${secs}s`;
}

/**
 * 将秒数格式化为人类可读时长。
 * @param totalSeconds 总秒数
 * @returns "2天 3小时" / "45分钟" / "—"
 */
export function formatDurationSeconds(totalSeconds: number): string {
	if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) return "—";
	const days = Math.floor(totalSeconds / 86400);
	const hours = Math.floor((totalSeconds % 86400) / 3600);
	const minutes = Math.floor((totalSeconds % 3600) / 60);
	const seconds = Math.floor(totalSeconds % 60);
	const parts: string[] = [];
	if (days > 0) parts.push(`${days}天`);
	if (hours > 0) parts.push(`${hours}小时`);
	if (minutes > 0) parts.push(`${minutes}分钟`);
	if (parts.length === 0 && seconds > 0) parts.push(`${seconds}秒`);
	return parts.join(" ") || "—";
}

/**
 * 根据时长秒数估算结束时间文案。
 */
export function formatMuteEndsAt(totalSeconds: number): string {
	if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) return "—";
	const ends = new Date(Date.now() + totalSeconds * 1000);
	const pad = (n: number) => String(n).padStart(2, "0");
	return `${ends.getMonth() + 1}/${ends.getDate()} ${pad(ends.getHours())}:${pad(ends.getMinutes())}`;
}
