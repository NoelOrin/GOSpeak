/**
 * 决定 /invite/join 的请求体：
 * 从公开域卡片进来时带 domain_uuid（免码，仍受域级审核约束）；
 * 从粘贴邀请码进来时带 code。两者都没有时返回 null，调用方应放弃请求。
 */
export function joinTarget(
	code: string,
	domainUUID: string,
): { code: string } | { domain_uuid: string } | null {
	if (domainUUID) return { domain_uuid: domainUUID };
	if (code) return { code };
	return null;
}
