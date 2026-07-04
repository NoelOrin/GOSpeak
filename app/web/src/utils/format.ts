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
