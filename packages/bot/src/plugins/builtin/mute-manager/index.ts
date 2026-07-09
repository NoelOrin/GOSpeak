/**
 * 禁言管理插件（Bot 有 mute:manage + user:read 权限时可用）
 *
 * 与 moderation 插件的 /mute（房间级客户端静音）不同，
 * 本插件操作的是服务端用户级禁言记录（POST /mute/create 等）。
 *
 * 命令：
 *   /gmute <user> <duration> [reason]   — 禁言用户（服务端记录）
 *   /gunmute <user>                     — 取消禁言（服务端记录）
 *   /gmute list                          — 列出生效禁言
 *   /gmute status <user>                 — 查询指定用户禁言状态
 *
 * duration 格式：1h / 30m / 7d / permanent / 数字秒数
 */
import { Plugin } from "../../../core/plugin";
import { RegisterPlugin } from "../../../decorators/register";
import { Command } from "../../../decorators/handlers";
import { PermissionFilter } from "../../../filters/index";
import type { MessageEvent } from "../../../core/types";
import type { GOSpeakApiClient } from "../../../runtime/apiClient";

interface MuteRecord {
	id: number;
	user_id: number;
	permanent: boolean;
	duration: number;
	reason: string;
	expires_at: string | null;
	created_at: string;
}

function parseDuration(input: string): { duration: number; permanent: boolean } | string {
	if (input === "permanent") {
		return { duration: 0, permanent: true };
	}
	const match = input.match(/^(\d+)([smhd])$/);
	if (match) {
		const num = parseInt(match[1], 10);
		switch (match[2]) {
			case "s": return { duration: num, permanent: false };
			case "m": return { duration: num * 60, permanent: false };
			case "h": return { duration: num * 3600, permanent: false };
			case "d": return { duration: num * 86400, permanent: false };
		}
	}
	const num = parseInt(input, 10);
	if (!isNaN(num) && num > 0) {
		return { duration: num, permanent: false };
	}
	return "invalid duration, use: 30m, 1h, 7d, permanent, or seconds";
}

function fmtRemaining(record: MuteRecord): string {
	if (record.permanent) return "permanent";
	if (!record.expires_at) return "unknown";
	const remaining = new Date(record.expires_at).getTime() - Date.now();
	if (remaining <= 0) return "expired";
	const secs = Math.floor(remaining / 1000);
	if (secs < 60) return `${secs}s`;
	if (secs < 3600) return `${Math.floor(secs / 60)}m`;
	if (secs < 86400) return `${Math.floor(secs / 3600)}h`;
	return `${Math.floor(secs / 86400)}d`;
}

@RegisterPlugin({
	name: "mute-manager",
	author: "gospeak",
	desc: "服务端禁言管理：禁言/解禁/列表/状态查询",
	version: "1.0.0",
})
export class MuteManagerPlugin extends Plugin {
	private get api(): GOSpeakApiClient {
		return (this.ctx as unknown as { chat: GOSpeakApiClient }).chat;
	}

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
				if (sub && args.length >= 1) {
					return this.handleCreate(event, sub, args);
				}
				await this.ctx.chat.reply(event,
					"用法: /gmute <user> <duration> [reason] | /gunmute <user> | /gmute list | /gmute status <user>");
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
			const userInfo = await this.api.getUserByIdentity(username);
			await this.api.unmuteUser(userInfo.id);
			await this.ctx.chat.reply(event, `已取消 ${username} 的禁言`);
		} catch (err) {
			await this.ctx.chat.reply(event, `取消失败: ${(err as Error).message}`);
		}
	}

	private async handleCreate(event: MessageEvent, username: string, args: string[]): Promise<void> {
		const durationStr = args[0];
		const reason = args.slice(1).join(" ") || "";

		const parsed = parseDuration(durationStr);
		if (typeof parsed === "string") {
			await this.ctx.chat.reply(event, parsed);
			return;
		}

		try {
			const userInfo = await this.api.getUserByIdentity(username);
			await this.api.muteUser(userInfo.id, parsed.duration, parsed.permanent, reason);
			await this.ctx.chat.reply(event, `已禁言 ${username} (${durationStr})${reason ? ` 原因: ${reason}` : ""}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `禁言失败: ${(err as Error).message}`);
		}
	}

	private async handleList(event: MessageEvent): Promise<void> {
		try {
			const mutes = await this.api.listMutes() as MuteRecord[];
			if (!mutes || mutes.length === 0) {
				await this.ctx.chat.reply(event, "当前没有生效禁言");
				return;
			}
			const lines = mutes.map(m => {
				const remaining = fmtRemaining(m);
				return `  - user#${m.user_id} ${remaining}${m.reason ? ` (${m.reason})` : ""}`;
			});
			await this.ctx.chat.reply(event, `生效禁言 (${mutes.length}):\n${lines.join("\n")}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `查询失败: ${(err as Error).message}`);
		}
	}

	private async handleStatus(event: MessageEvent, args: string[]): Promise<void> {
		const username = args[0];
		if (!username) {
			await this.ctx.chat.reply(event, "用法: /gmute status <user>");
			return;
		}
		try {
			const userInfo = await this.api.getUserByIdentity(username);
			const status = await this.api.getMuteStatus(userInfo.id) as MuteRecord | null;
			if (!status) {
				await this.ctx.chat.reply(event, `${username} 未被禁言`);
				return;
			}
			const remaining = fmtRemaining(status);
			await this.ctx.chat.reply(event,
				`${username} 禁言中: ${remaining}${status.reason ? `, 原因: ${status.reason}` : ""}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `查询失败: ${(err as Error).message}`);
		}
	}
}
