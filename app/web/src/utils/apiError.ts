/** 从任意错误里提取后端 `Result.msg`，取不到时退回 Error.message / 兜底文案。 */
export function apiErrorMessage(error: unknown, fallback = "请求失败"): string {
	const response = (error as { response?: { data?: { msg?: string } } })
		?.response?.data?.msg;
	if (response) return response;
	if (error instanceof Error && error.message) return error.message;
	return fallback;
}

/** 后端业务码 + 文案，用于按 code 定制提示。 */
export function apiErrorCode(error: unknown): number | undefined {
	return (error as { response?: { data?: { code?: number } } })?.response?.data
		?.code;
}
