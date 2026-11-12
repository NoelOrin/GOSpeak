function toDate(value: string | number | null | undefined): Date | null {
	if (value === null || value === undefined || value === "") return null;
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) return null;
	return date;
}

function pad(value: number): string {
	return value < 10 ? `0${value}` : String(value);
}

function startOfDay(date: Date): number {
	return new Date(
		date.getFullYear(),
		date.getMonth(),
		date.getDate(),
	).getTime();
}

/** 日期分隔条文案：2026年11月10日 */
export function formatDateLabel(
	value: string | number | null | undefined,
): string {
	const date = toDate(value);
	if (!date) return "";
	return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日`;
}

/**
 * 消息行时间文案：
 * 今天 -> HH:mm；昨天 -> 昨天 HH:mm；更早 -> M月D日 HH:mm
 */
export function formatMessageTime(
	value: string | number | null | undefined,
): string {
	const date = toDate(value);
	if (!date) return "";
	const clock = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
	const today = startOfDay(new Date());
	const day = startOfDay(date);
	if (day === today) return clock;
	if (today - day === 24 * 60 * 60 * 1000) return `昨天 ${clock}`;
	return `${date.getMonth() + 1}月${date.getDate()}日 ${clock}`;
}

/** 用于分组的稳定键：YYYY-MM-DD（本地时区） */
export function dayKey(value: string | number | null | undefined): string {
	const date = toDate(value);
	if (!date) return "";
	return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}
