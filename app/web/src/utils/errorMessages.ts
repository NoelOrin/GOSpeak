import { apiErrorCode, apiErrorMessage } from "./apiError";

/**
 * 后端英文 msg → 前端中文文案。
 *
 * 后端的错误文案是给开发者看的（"room is full (%d/%d)"、"wrong room password"），
 * 直接透传到界面会让用户看到中英混排甚至裸英文。这里按"归一化后的英文原文"查表，
 * 命中就换中文，未命中的保留原文（方便排查），但调用方仍应给一个兜底文案。
 *
 * 归一化规则：去掉末尾的动态参数（括号内容 / 数字），全部小写。
 */
const SERVER_MESSAGE_ZH: Record<string, string> = {
	// 语音 / 房间
	"room is full": "房间已满",
	"room requires password": "该房间需要密码",
	"wrong room password": "房间密码错误",
	"room password required": "该房间需要密码",
	"room not found": "房间不存在或已被删除",
	"room does not exist": "房间不存在或已被删除",
	"duplicate connection not allowed": "该账号已在其他设备加入此房间",
	"kicked, try again later": "你已被移出该房间，请稍后再试",
	"sfu not configured": "语音服务尚未配置，请联系管理员",
	"sfu error": "语音服务异常，请稍后重试",
	"not in a room": "你不在该房间内",
	// 域
	"domain not found": "域不存在或已被删除",
	"invite code is invalid": "邀请码无效",
	"invalid invite code": "邀请码无效",
	"user is already a member": "你已经是该域成员",
	"already a member": "你已经是该域成员",
	forbidden: "没有操作权限",
	"insufficient domain message permission": "你在该域没有查看消息的权限",
	// 账号
	"invalid credentials": "用户名或密码错误",
	"user not found": "用户不存在",
	"username already exists": "用户名已被占用",
	"token has expired": "登录已过期，请重新登录",
	"token has been revoked": "登录已失效，请重新登录",
	"user has been banned": "账号已被封禁",
	"user has been muted": "你已被禁言",
	"too many requests": "操作过于频繁，请稍后再试",
	// 通用
	"internal server error": "服务器开小差了，请稍后重试",
	"not found": "资源不存在",
	"already exists": "资源已存在",
	"invalid parameters": "参数不正确",
	"messages missing": "消息加载失败",
	// 邀请码 / 加入审核
	"invite code format is invalid": "邀请链接无效",
	"invite code has expired": "邀请链接已过期",
	"invite code is disabled": "邀请链接已停用",
	"invite code has reached its usage limit": "邀请链接已达使用上限",
	"invite code is no longer usable": "邀请链接已不可用",
	"invite code is required": "请填写邀请码",
	"this domain requires an invite code": "该域需要邀请码才能加入",
	"missing domain member review permission": "你没有审核权限",
	"missing domain invite manage permission": "你没有邀请码管理权限",
	"join request not found": "加入申请不存在",
	"join request has already been reviewed": "该申请已被其他管理员处理",
	"join request is not pending": "该申请不在待审核状态",
	"applicant no longer exists": "申请人账号已不存在",
	"nickname is too long": "昵称过长",
	"email and email_code are required":
		"该服务器已开启邮箱验证，请填写邮箱与验证码",
	"password must be at least 8 characters": "密码至少 8 位",
	"domain_uuid and user_uuid are required": "缺少必要的参数",
};

function normalize(raw: string): string {
	return raw
		.toLowerCase()
		.replace(/\([^)]*\)/g, "") // room is full (3/10) → room is full
		.replace(/[：:]\s*\d+\s*\/\s*\d+/g, "")
		.replace(/\s+/g, " ")
		.trim();
}

/**
 * 把 axios 错误翻译成面向用户的中文文案。
 * 永远返回可直接展示的文案：命中词表用中文，否则用 fallback，最后才退回原文。
 */
export function localizedApiMessage(
	error: unknown,
	fallback = "操作失败",
): string {
	const raw = apiErrorMessage(error, "");
	if (!raw) return fallback;
	const key = normalize(raw);
	const hit = SERVER_MESSAGE_ZH[key];
	if (hit) return hit;
	// 后端偶尔会在原文里带前缀，做一次包含匹配兜底
	for (const [en, zh] of Object.entries(SERVER_MESSAGE_ZH)) {
		if (key.includes(en)) return zh;
	}
	return fallback;
}

/** 是否是房间密码相关错误（需要提示用户重新输入密码）。 */
export function isRoomPasswordError(error: unknown): boolean {
	const raw = apiErrorMessage(error, "");
	return /password required|wrong room password|room requires password/i.test(
		raw,
	);
}

/** 业务码是否属于"权限 / 鉴权"类，用于决定弹哪个提示。 */
export function isForbiddenError(error: unknown): boolean {
	const code = apiErrorCode(error);
	return code === 1013 || code === 1015;
}
