import apiClient from "./apiClient";

/** 服务端版本信息（构建时注入，dev 为源码构建）。 */
export interface ServerVersionInfo {
	version: string;
	commit: string;
}

/** 服务端更新检查结果（与桌面端更新体系独立，仅主体 v* 发布）。 */
export interface ServerUpdateInfo {
	/** 运行中服务端的版本。 */
	current: string;
	/** 最新正式发布版本号；GitHub 请求失败时为空。 */
	latest: string;
	/** 最新版本比当前版本新。 */
	available: boolean;
	/** 最新版本的发布说明（Markdown，超长截断）。 */
	notes?: string;
	/** 发布页链接。 */
	url?: string;
}

/** 当前服务端版本（无网络请求，仅 admin）。 */
export async function getServerVersion(): Promise<ServerVersionInfo> {
	const data = await apiClient.get<ServerVersionInfo>({
		url: "/api/v1/system/update/current",
	});
	return (
		data || {
			version: "unknown",
			commit: "unknown",
		}
	);
}

/**
 * 当前服务端版本（公开只读端点，无需登录）。
 *
 * 与 getServerVersion 的区别只在权限：那个走 /system/update/current（admin），
 * 这个走 /system/version，供「关于」页给所有用户展示后端版本。
 */
export async function getPublicServerVersion(): Promise<ServerVersionInfo> {
	const data = await apiClient.get<ServerVersionInfo>({
		url: "/api/v1/system/version",
		// 版本展示属非关键路径，静默失败由调用方回退成「未知」，不弹全局 toast。
		skipErrorToast: true,
	});
	return (
		data || {
			version: "unknown",
			commit: "unknown",
		}
	);
}

/** 检查服务端更新（读 GitHub Releases，带服务端缓存；仅 admin）。 */
export async function checkServerUpdate(): Promise<ServerUpdateInfo> {
	const data = await apiClient.get<ServerUpdateInfo>({
		url: "/api/v1/system/update/check",
		// 失败（多为 GitHub 网络问题）由调用方在卡片内展示，不走全局 toast。
		skipErrorToast: true,
	});
	// data 缺失说明响应异常（网关改写、旧后端、code 0 但 data null）。绝不能兜底成
	// { available: false }：那会让监控卡片渲染成「已是最新版本」的成功态。
	if (!data) throw new Error("服务端更新检查返回了空响应");
	return data;
}
