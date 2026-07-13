/**
 * 新成员欢迎插件
 *
 * 功能：
 * - 新成员加入房间时发送欢迎消息
 * - 支持自定义欢迎语模板（/welcome set <message>）
 * - 支持开关欢迎功能（/welcome on|off）
 * - 欢迎语支持 {name} 占位符
 */
import { Plugin } from "../../../core/plugin";
import type { MessageEvent, RoomEvent } from "../../../core/types";
import { EventType } from "../../../core/types";
import { Command, On } from "../../../decorators/handlers";
import { RegisterPlugin } from "../../../decorators/register";
import { PermissionFilter } from "../../../filters/index";

const KV_TEMPLATE = "welcome:template";
const KV_ENABLED = "welcome:enabled";
const DEFAULT_TEMPLATE = "欢迎 {name} 加入语音频道！";

@RegisterPlugin({
	name: "welcome",
	author: "gospeak",
	desc: "新成员欢迎：自定义模板 + 开关",
	version: "1.0.0",
})
export class WelcomePlugin extends Plugin {
	private template: string = DEFAULT_TEMPLATE;
	private enabled: boolean = true;

	async onLoad(): Promise<void> {
		const tpl = await this.ctx.kv.get<string>(KV_TEMPLATE);
		if (tpl) this.template = tpl;
		const en = await this.ctx.kv.get<boolean>(KV_ENABLED);
		if (en !== undefined) this.enabled = en;
	}

	@Command("welcome", {
		desc: "欢迎消息管理",
		filters: [new PermissionFilter("moderator")],
	})
	async onWelcome(event: MessageEvent): Promise<void> {
		const sub = event.rawCommand?.args[0];
		const args = event.rawCommand?.args.slice(1) ?? [];

		switch (sub) {
			case "set":
				return this.handleSet(event, args);
			case "on":
				this.enabled = true;
				await this.ctx.kv.set(KV_ENABLED, true);
				await this.ctx.chat.reply(event, "欢迎功能已开启");
				return;
			case "off":
				this.enabled = false;
				await this.ctx.kv.set(KV_ENABLED, false);
				await this.ctx.chat.reply(event, "欢迎功能已关闭");
				return;
			case "show":
				await this.ctx.chat.reply(event, `当前欢迎语: ${this.template}`);
				return;
			default:
				await this.ctx.chat.reply(event, "用法: /welcome set|on|off|show");
		}
	}

	private async handleSet(event: MessageEvent, args: string[]): Promise<void> {
		const msg = args.join(" ");
		if (!msg) {
			await this.ctx.chat.reply(
				event,
				"用法: /welcome set <message>（可用 {name} 占位）",
			);
			return;
		}
		this.template = msg;
		await this.ctx.kv.set(KV_TEMPLATE, msg);
		await this.ctx.chat.reply(event, `欢迎语已更新: ${msg}`);
	}

	@On(EventType.OnRoomJoined, { desc: "发送欢迎消息" })
	async onMemberJoined(event: RoomEvent): Promise<void> {
		if (!this.enabled || !event.actor) return;
		const message = this.template.replace("{name}", event.actor.name);
		await this.ctx.chat.send(event.room.id, message);
	}
}
