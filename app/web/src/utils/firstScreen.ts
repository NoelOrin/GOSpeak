/**
 * 首屏目标域：优先回到上次所在域，但必须仍在我的域列表里
 * （可能已经退域、被移出，或换了账号导致本地记录过期）。
 *
 * @returns 目标域 UUID；没有任何已加入的域时返回 null，交给发现页引导创建/加入。
 */
export function resolveFirstScreen(
	myDomainUUIDs: readonly string[],
	lastDomainUUID: string | null,
): string | null {
	if (myDomainUUIDs.length === 0) return null;
	if (lastDomainUUID && myDomainUUIDs.includes(lastDomainUUID)) {
		return lastDomainUUID;
	}
	return myDomainUUIDs[0];
}
