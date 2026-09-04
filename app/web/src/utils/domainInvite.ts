const INVITE_CODE_PATTERN = /^[A-Z2-9]{8}$/i;
const INVITE_LINK_PATTERN = /\/invite\/d\/([A-Z2-9]{8})/i;

export function extractDomainInviteCode(input: string): string | null {
	const text = input.trim();
	if (!text) return null;

	if (INVITE_CODE_PATTERN.test(text)) {
		return text.toUpperCase();
	}

	const linkMatch = text.match(INVITE_LINK_PATTERN);
	if (linkMatch?.[1]) {
		return linkMatch[1].toUpperCase();
	}

	try {
		const url = new URL(text);
		const code =
			url.searchParams.get("code") || url.searchParams.get("invite_code");
		if (code && INVITE_CODE_PATTERN.test(code)) {
			return code.toUpperCase();
		}
	} catch {
		// 非 URL 文本，直接按邀请码规则处理。
	}

	return null;
}

export function domainInviteUrl(code: string): string {
	return `${window.location.origin}/invite/d/${encodeURIComponent(code)}`;
}
