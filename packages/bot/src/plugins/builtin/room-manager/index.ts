/**
 * 语音频道自动管理插件
 *
 * 功能：
 * - /room create <name> [limit]  — 创建房间
 * - /room list                   — 列出活跃房间
 * - /room members                — 列出当前房间成员
 * - /room limit <number>         — 设置房间人数上限（管理员）
 * - 自动欢迎新加入成员
 * - 房间空时自动通知
 */
import { Plugin } from "../../../core/plugin";
import { RegisterPlugin } from "../../../decorators/register";
import { Command, On } from "../../../decorators/handlers";
import { PermissionFilter } from "../../../filters/index";
import { EventType } from "../../../core/types";
import type { MessageEvent, RoomEvent } from "../../../core/types";

@RegisterPlugin({
	name: "room-manager",
	author: "gospeak",
	desc: "语音频道自动管理：创建/列表/成员/上限 + 自动欢迎",
	version: "1.0.0",
})
export class RoomManagerPlugin extends Plugin {
	@Command("room", { alias: ["r"], desc: "语音频道管理" })
	async onRoom(event: MessageEvent): Promise<void> {
		const sub = event.rawCommand?.args[0];
		const args = event.rawCommand?.args.slice(1) ?? [];

		switch (sub) {
			case "create":
				return this.handleCreate(event, args);
			case "list":
				return this.handleList(event);
			case "members":
				return this.handleMembers(event);
			case "limit":
				return this.handleLimit(event, args);
			default:
				await this.ctx.chat.reply(event, "用法: /room create|list|members|limit");
		}
	}

	private async handleCreate(event: MessageEvent, args: string[]): Promise<void> {
		const name = args[0];
		if (!name) {
			await this.ctx.chat.reply(event, "用法: /room create <name> [limit]");
			return;
		}
		const limit = args[1] ? parseInt(args[1], 10) : undefined;
		try {
			const room = await this.ctx.rooms.createRoom(name, limit);
			await this.ctx.chat.reply(event, `房间已创建: ${room.name}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `创建失败: ${(err as Error).message}`);
		}
	}

	private async handleList(event: MessageEvent): Promise<void> {
		try {
			const rooms = await this.ctx.rooms.listRooms();
			if (rooms.length === 0) {
				await this.ctx.chat.reply(event, "当前没有活跃房间");
				return;
			}
			const lines = rooms.map((r) => `  - ${r.name} (${r.id})`);
			await this.ctx.chat.reply(event, `活跃房间:\n${lines.join("\n")}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `查询失败: ${(err as Error).message}`);
		}
	}

	private async handleMembers(event: MessageEvent): Promise<void> {
		try {
			const members = await this.ctx.rooms.getMembers(event.room.id);
			if (members.length === 0) {
				await this.ctx.chat.reply(event, "当前房间没有成员");
				return;
			}
			const lines = members.map(
				(m) => `  - ${m.name} [${m.role}]${m.identity === event.sender.identity ? " (你)" : ""}`,
			);
			await this.ctx.chat.reply(event, `房间成员 (${members.length}):\n${lines.join("\n")}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `查询失败: ${(err as Error).message}`);
		}
	}

	@Command("room", {
		alias: ["r"],
		desc: "设置房间人数上限（管理员）",
		filters: [new PermissionFilter("admin")],
	})
	private async handleLimit(event: MessageEvent, args: string[]): Promise<void> {
		const limitStr = args[0];
		if (!limitStr) {
			await this.ctx.chat.reply(event, "用法: /room limit <number>");
			return;
		}
		const limit = parseInt(limitStr, 10);
		if (isNaN(limit) || limit < 1) {
			await this.ctx.chat.reply(event, "人数上限必须为正整数");
			return;
		}
		await this.ctx.kv.set(`room:${event.room.id}:limit`, limit);
		await this.ctx.chat.reply(event, `房间 ${event.room.name} 人数上限设为 ${limit}`);
	}

	@On(EventType.OnRoomJoined, { desc: "自动欢迎新成员" })
	async onMemberJoined(event: RoomEvent): Promise<void> {
		if (!event.actor) return;
		await this.ctx.chat.send(
			event.room.id,
			`${event.actor.name} 加入了房间`,
		);
	}

	@On(EventType.OnRoomLeft, { desc: "成员离开通知" })
	async onMemberLeft(event: RoomEvent): Promise<void> {
		if (!event.actor) return;
		await this.ctx.chat.send(
			event.room.id,
			`${event.actor.name} 离开了房间`,
		);
	}
}
