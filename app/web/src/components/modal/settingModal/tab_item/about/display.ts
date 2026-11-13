/**
 * 版本号展示口径。
 *
 * 空值与后端兜底的 "unknown" 都视为「未知」；dev 源码构建照常显示 dev，
 * 这是有效信息，不能吞掉。
 */
export function displayVersion(raw?: string | null): string {
	const value = (raw ?? "").trim();
	if (!value || value === "unknown") return "未知";
	return value;
}
