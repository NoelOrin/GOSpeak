/**
 * 关键词自动回复插件
 *
 * 功能：
 * - 基于配置的关键词映射表自动回复消息
 * - 支持 /keyword add <trigger> <response>  — 添加关键词
 * - 支持 /keyword remove <trigger>          — 删除关键词
 * - 支持 /keyword list                      — 列出所有关键词
 * - 关键词存储在 KV 中，跨重启持久化
 */
import { Plugin } from "../../../core/plugin";
import { RegisterPlugin } from "../../../decorators/register";
import { Command, On } from "../../../decorators/handlers";
import { PermissionFilter } from "../../../filters/index";
import { EventType } from "../../../core/types";
import type { MessageEvent } from "../../../core/types";

interface KeywordMap {
	[trigger: string]: string;
}

const KV_KEY = "keyword-reply:map";

@RegisterPlugin({
	name: "keyword-reply",
	author: "gospeak",
	desc: "关键词自动回复：可配置触发词与回复内容",
	version: "1.0.0",
})
export class KeywordReplyPlugin extends Plugin {
	private keywords: KeywordMap = {};

	async onLoad(): Promise<void> {
		const stored = await this.ctx.kv.get<KeywordMap>(KV_KEY);
		this.keywords = stored ?? {};
		this.ctx.logger.info(`keyword-reply: loaded ${Object.keys(this.keywords).length} keywords`);
	}

	private async save(): Promise<void> {
		await this.ctx.kv.set(KV_KEY, this.keywords);
	}

	@Command("keyword", {
		alias: ["kw"],
		desc: "关键词管理",
		filters: [new PermissionFilter("moderator")],
	})
	async onKeyword(event: MessageEvent): Promise<void> {
		const sub = event.rawCommand?.args[0];
		const args = event.rawCommand?.args.slice(1) ?? [];

		switch (sub) {
			case "add":
				return this.handleAdd(event, args);
			case "remove":
				return this.handleRemove(event, args);
			case "list":
				return this.handleList(event);
			default:
				await this.ctx.chat.reply(event, "用法: /keyword add|remove|list");
		}
	}

	private async handleAdd(event: MessageEvent, args: string[]): Promise<void> {
		const trigger = args[0];
		const response = args.slice(1).join(" ");
		if (!trigger || !response) {
			await this.ctx.chat.reply(event, "用法: /keyword add <trigger> <response>");
			return;
		}
		this.keywords[trigger.toLowerCase()] = response;
		await this.save();
		await this.ctx.chat.reply(event, `已添加关键词: ${trigger}`);
	}

	private async handleRemove(event: MessageEvent, args: string[]): Promise<void> {
		const trigger = args[0];
		if (!trigger) {
			await this.ctx.chat.reply(event, "用法: /keyword remove <trigger>");
			return;
		}
		const key = trigger.toLowerCase();
		if (!(key in this.keywords)) {
			await this.ctx.chat.reply(event, `关键词不存在: ${trigger}`);
			return;
		}
		delete this.keywords[key];
		await this.save();
		await this.ctx.chat.reply(event, `已删除关键词: ${trigger}`);
	}

	private async handleList(event: MessageEvent): Promise<void> {
		const keys = Object.keys(this.keywords);
		if (keys.length === 0) {
			await this.ctx.chat.reply(event, "当前没有配置关键词");
			return;
		}
		const lines = keys.map((k) => `  ${k} → ${this.keywords[k]}`);
		await this.ctx.chat.reply(event, `关键词列表:\n${lines.join("\n")}`);
	}

	@On(EventType.OnMessageReceived, {
		desc: "匹配关键词并自动回复",
		priority: -10,
	})
	async onMessage(event: MessageEvent): Promise<void> {
		if (event.isCommand) return;
		const content = event.content.toLowerCase();
		for (const [trigger, response] of Object.entries(this.keywords)) {
			if (content.includes(trigger)) {
				await this.ctx.chat.reply(event, response);
				return;
			}
		}
	}
}
