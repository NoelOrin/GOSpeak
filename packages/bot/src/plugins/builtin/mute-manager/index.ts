/**
 * 禁言管理插件 — 使用 ctx.mutes API（Phase 1）。
 *
 * 命令：
 *   /gmute <user> <duration> [reason]   — 禁言用户
 *   /gunmute <user>                     — 取消禁言
 *   /gmute list                          — 列出生效禁言
 *   /gmute status <user>                 — 查询禁言状态
 *
 * duration：1h / 30m / 7d / permanent / 秒数
 */
import { Plugin } from "../../../core/plugin";
import type { MessageEvent } from "../../../core/types";
import { Command } from "../../../decorators/handlers";
import { RegisterPlugin } from "../../../decorators/register";
import { PermissionFilter } from "../../../filters/index";

interface MuteRecord {
	id: number;
	user_id: number;
	permanent: boolean;
	duration: number;
	reason: string;
	expires_at: string | null;
	created_at: string;
}

function parseDuration(
	input: string,
): { duration: number; permanent: boolean } | string {
	if (input === "permanent") return { duration: 0, permanent: true };
	const m = input.match(/^(\d+)([smhd])$/);
	if (m) {
		const n = parseInt(m[1], 10);
		switch (m[2]) {
			case "s":
				return { duration: n, permanent: false };
			case "m":
				return { duration: n * 60, permanent: false };
			case "h":
				return { duration: n * 3600, permanent: false };
			case "d":
				return { duration: n * 86400, permanent: false };
		}
	}
	const n = parseInt(input, 10);
	if (!Number.isNaN(n) && n > 0) return { duration: n, permanent: false };
	return "invalid duration";
}

function fmtRemaining(r: MuteRecord): string {
	if (r.permanent) return "permanent";
	if (!r.expires_at) return "unknown";
	const rem = new Date(r.expires_at).getTime() - Date.now();
	if (rem <= 0) return "expired";
	const s = Math.floor(rem / 1000);
	if (s < 60) return `${s}s`;
	if (s < 3600) return `${Math.floor(s / 60)}m`;
	if (s < 86400) return `${Math.floor(s / 3600)}h`;
	return `${Math.floor(s / 86400)}d`;
}

@RegisterPlugin({
	name: "mute-manager",
	author: "gospeak",
	desc: "服务端禁言管理：禁言/解禁/列表/状态查询",
	version: "1.0.0",
})
export class MuteManagerPlugin extends Plugin {
	@Command("gmute", {
		desc: "服务端禁言管理",
		filters: [new PermissionFilter("admin")],
	})
	async onGmute(event: MessageEvent): Promise<void> {
		const sub = event.rawCommand?.args[0];
		const args = event.rawCommand?.args.slice(1) ?? [];
		switch (sub) {
			case "list":
				return this.handleList(event);
			case "status":
				return this.handleStatus(event, args);
			default:
				if (sub && args.length >= 1) return this.handleCreate(event, sub, args);
				await this.ctx.chat.reply(
					event,
					"用法: /gmute <user> <duration> [reason] | /gunmute <user> | /gmute list | /gmute status <user>",
				);
		}
	}

	@Command("gunmute", {
		desc: "取消服务端禁言",
		filters: [new PermissionFilter("admin")],
	})
	async onGunmute(event: MessageEvent): Promise<void> {
		const username = event.rawCommand?.args[0];
		if (!username) {
			await this.ctx.chat.reply(event, "用法: /gunmute <user>");
			return;
		}
		try {
			const user = await this.ctx.users.getByIdentity(username);
			await this.ctx.voice.muteMember(event.room.id, username, false);
			await this.ctx.chat.reply(event, `已取消 ${username} 的禁言`);
		} catch (err) {
			await this.ctx.chat.reply(event, `取消失败: ${(err as Error).message}`);
		}
	}

	private async handleCreate(
		event: MessageEvent,
		username: string,
		args: string[],
	): Promise<void> {
		const durationStr = args[0];
		const reason = args.slice(1).join(" ") || "";
		const parsed = parseDuration(durationStr);
		if (typeof parsed === "string") {
			await this.ctx.chat.reply(event, parsed);
			return;
		}
		try {
			await this.ctx.users.getByIdentity(username);
			await this.ctx.voice.muteMember(event.room.id, username, true);
			await this.ctx.chat.reply(
				event,
				`已禁言 ${username} (${durationStr})${reason ? ` 原因: ${reason}` : ""}`,
			);
		} catch (err) {
			await this.ctx.chat.reply(event, `禁言失败: ${(err as Error).message}`);
		}
	}

	private async handleList(event: MessageEvent): Promise<void> {
		try {
			const mutes = (await this.ctx.mutes.list()) as MuteRecord[];
			if (!mutes || mutes.length === 0) {
				await this.ctx.chat.reply(event, "当前没有生效禁言");
				return;
			}
			const lines = mutes.map(
				(m) =>
					`  - user#${m.user_id} ${fmtRemaining(m)}${m.reason ? ` (${m.reason})` : ""}`,
			);
			await this.ctx.chat.reply(
				event,
				`生效禁言 (${mutes.length}):\n${lines.join("\n")}`,
			);
		} catch (err) {
			await this.ctx.chat.reply(event, `查询失败: ${(err as Error).message}`);
		}
	}

	private async handleStatus(
		event: MessageEvent,
		args: string[],
	): Promise<void> {
		const username = args[0];
		if (!username) {
			await this.ctx.chat.reply(event, "用法: /gmute status <user>");
			return;
		}
		try {
			const userInfo = await this.ctx.users.getByIdentity(username);
			const status = (await this.ctx.mutes.status(
				userInfo.id,
			)) as MuteRecord | null;
			if (!status) {
				await this.ctx.chat.reply(event, `${username} 未被禁言`);
				return;
			}
			await this.ctx.chat.reply(
				event,
				`${username} 禁言中: ${fmtRemaining(status)}${status.reason ? `, 原因: ${status.reason}` : ""}`,
			);
		} catch (err) {
			await this.ctx.chat.reply(event, `查询失败: ${(err as Error).message}`);
		}
	}
}
