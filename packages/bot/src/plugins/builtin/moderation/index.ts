/**
 * 房间管理插件（审核/控制）
 *
 * 功能：
 * - /kick <identity>              — 踢出成员（管理员）
 * - /mute <identity> [duration]   — 禁言成员（管理员）
 * - /unmute <identity>            — 解除禁言（管理员）
 * - /volume <identity> <0-100>    — 设置成员音量（管理员）
 * - 自动检测长时间静音成员并提醒
 */
import { Plugin } from "../../../core/plugin";
import type { MemberStateEvent, MessageEvent } from "../../../core/types";
import { EventType } from "../../../core/types";
import { Command, On } from "../../../decorators/handlers";
import { RegisterPlugin } from "../../../decorators/register";
import { PermissionFilter } from "../../../filters/index";

@RegisterPlugin({
	name: "moderation",
	author: "gospeak",
	desc: "房间审核控制：踢人/禁言/音量 + 静音检测",
	version: "1.0.0",
})
export class ModerationPlugin extends Plugin {
	private mutedSince = new Map<string, number>();
	private readonly muteWarnThresholdMs = 5 * 60 * 1000;

	@Command("kick", {
		desc: "踢出成员（管理员）",
		filters: [new PermissionFilter("admin")],
	})
	async onKick(event: MessageEvent): Promise<void> {
		const target = event.rawCommand?.args[0];
		if (!target) {
			await this.ctx.chat.reply(event, "用法: /kick <identity>");
			return;
		}
		try {
			await this.ctx.voice.removeMember(event.room.id, target);
			await this.ctx.chat.reply(event, `已踢出: ${target}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `操作失败: ${(err as Error).message}`);
		}
	}

	@Command("mute", {
		desc: "禁言成员（管理员）",
		filters: [new PermissionFilter("admin")],
	})
	async onMute(event: MessageEvent): Promise<void> {
		const target = event.rawCommand?.args[0];
		if (!target) {
			await this.ctx.chat.reply(event, "用法: /mute <identity> [duration秒]");
			return;
		}
		const duration = event.rawCommand?.args[1]
			? parseInt(event.rawCommand.args[1], 10)
			: 0;
		try {
			await this.ctx.voice.muteMember(event.room.id, target, true);
			const msg =
				duration > 0
					? `已禁言: ${target} (${duration}秒)`
					: `已禁言: ${target}`;
			await this.ctx.chat.reply(event, msg);
			if (duration > 0) {
				setTimeout(async () => {
					try {
						await this.ctx.voice.muteMember(event.room.id, target, false);
						await this.ctx.chat.send(
							event.room.id,
							`已自动解除禁言: ${target}`,
						);
					} catch {
						// member may have left
					}
				}, duration * 1000);
			}
		} catch (err) {
			await this.ctx.chat.reply(event, `操作失败: ${(err as Error).message}`);
		}
	}

	@Command("unmute", {
		desc: "解除禁言（管理员）",
		filters: [new PermissionFilter("admin")],
	})
	async onUnmute(event: MessageEvent): Promise<void> {
		const target = event.rawCommand?.args[0];
		if (!target) {
			await this.ctx.chat.reply(event, "用法: /unmute <identity>");
			return;
		}
		try {
			await this.ctx.voice.muteMember(event.room.id, target, false);
			await this.ctx.chat.reply(event, `已解除禁言: ${target}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `操作失败: ${(err as Error).message}`);
		}
	}

	@Command("volume", {
		desc: "设置成员音量（管理员）",
		filters: [new PermissionFilter("admin")],
	})
	async onVolume(event: MessageEvent): Promise<void> {
		const target = event.rawCommand?.args[0];
		const volStr = event.rawCommand?.args[1];
		if (!target || !volStr) {
			await this.ctx.chat.reply(event, "用法: /volume <identity> <0-100>");
			return;
		}
		const volume = parseInt(volStr, 10);
		if (Number.isNaN(volume) || volume < 0 || volume > 100) {
			await this.ctx.chat.reply(event, "音量必须为 0-100");
			return;
		}
		try {
			await this.ctx.voice.setMemberVolume(event.room.id, target, volume);
			await this.ctx.chat.reply(event, `${target} 音量设为 ${volume}`);
		} catch (err) {
			await this.ctx.chat.reply(event, `操作失败: ${(err as Error).message}`);
		}
	}

	@On(EventType.OnMemberStateChanged, { desc: "静音状态追踪与提醒" })
	async onMemberState(event: MemberStateEvent): Promise<void> {
		const key = `${event.room.id}:${event.member.identity}`;
		if (event.muted) {
			if (!this.mutedSince.has(key)) {
				this.mutedSince.set(key, Date.now());
			}
		} else {
			this.mutedSince.delete(key);
		}
	}

	@On(EventType.OnMemberLeft, { desc: "清理离开成员的静音状态" })
	async onMemberLeft(event: MemberEvent): Promise<void> {
		if (!event.actor) return;
		const key = `${event.room.id}:${event.actor.identity}`;
		this.mutedSince.delete(key);
	}

	/** 定期检查长时间静音成员（由 BotRunner 调用或通过定时器） */
	checkLongMuted(roomId: string): string[] {
		const now = Date.now();
		const warnings: string[] = [];
		for (const [key, since] of this.mutedSince) {
			if (
				key.startsWith(`${roomId}:`) &&
				now - since > this.muteWarnThresholdMs
			) {
				const identity = key.split(":")[1];
				warnings.push(identity);
			}
		}
		return warnings;
	}
}

type RoomEvent = import("../../../core/types").RoomEvent;
type MemberEvent = RoomEvent;
