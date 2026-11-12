import { z } from "zod";

export function fieldError(
	schema: z.ZodTypeAny,
	value: unknown,
): string | undefined {
	const result = schema.safeParse(value);
	return result.success ? undefined : result.error.issues[0]?.message;
}

export const requiredString = (message: string) => z.string().min(1, message);

export const newPasswordSchema = z
	.string()
	.min(1, "新密码 是必填项")
	.min(6, "密码长度不能少于6位");

export const roomNameSchema = z
	.string()
	.trim()
	.min(2, "房间名称至少需要 2 个字符")
	.max(32, "房间名称不能超过 32 个字符");

export const roomDescriptionSchema = z
	.string()
	.trim()
	.max(120, "房间说明不能超过 120 个字符");

export const roomPasswordSchema = z
	.string()
	.max(32, "房间密码不能超过 32 个字符");

export const domainNameSchema = z.string().trim().min(1, "域名称不能为空");

export const editRoomLimitSchema = z
	.custom<number>(
		(value) => typeof value === "number" && !Number.isNaN(value),
		{ message: "人数上限至少为 2" },
	)
	.refine((value) => value >= 2, "人数上限至少为 2")
	.refine((value) => value <= 200, "人数上限不能超过 200");

export const createRoomLimitSchema = z
	.custom<number>(
		(value) => typeof value === "number" && !Number.isNaN(value),
		{ message: "请填写人数上限" },
	)
	.refine((value) => value >= 2, "人数上限至少为 2")
	.refine((value) => value <= 200, "人数上限不能超过 200");

export const editRoomSchema = z.object({
	name: roomNameSchema,
	description: roomDescriptionSchema,
	limit: editRoomLimitSchema,
});
