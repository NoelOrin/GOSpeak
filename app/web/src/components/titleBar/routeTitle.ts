/** 只取 TanStack Router 匹配结果里用得到的部分，staticData 在各页面自行声明。 */
export interface RouteMatchLike {
	staticData?: unknown;
}

// 兜底映射：只在路由未配置 staticData.title、且路径段不是动态参数时使用
export const SEGMENT_TITLES: Record<string, string> = {
	discover: "发现域",
	chat: "聊天",
	link: "分享链接",
	profile: "个人资料",
	guest: "访客访问",
	login: "登录",
	register: "注册",
	manage: "管理",
	domain: "语音域",
	domains: "域管理",
	users: "用户管理",
	roles: "角色权限",
	mute: "禁言",
	ban: "封禁",
	sfu: "SFU 管理",
	storage: "存储",
	email: "邮箱",
	oauth: "OAuth",
	monitor: "服务监控",
};

/**
 * 推导窗口标题：优先取路由链上最近一级的 staticData.title（各页面已配置中文名），
 * 避免把 /manage/domains 的末段原样显示成 "domains"；动态段（UUID / 数字 ID）
 * 不作为标题，回退到父级路径段；都没有时回退 GOSpeak。
 */
export function resolveRouteTitle(
	matches: readonly RouteMatchLike[],
	pathname: string,
): string {
	for (let i = matches.length - 1; i >= 0; i--) {
		const data = matches[i].staticData as { title?: string } | undefined;
		if (data?.title) return data.title;
	}
	const segments = pathname.split("/").filter(Boolean);
	const last = segments.at(-1);
	const fallback =
		last && /^[0-9a-f-]{8,}$/i.test(last) ? segments.at(-2) : last;
	return (fallback ? SEGMENT_TITLES[fallback] : undefined) ?? "GOSpeak";
}
