/**
 * Listen room manager plugin.
 *
 * /listen add <room>
 * /listen remove <room>
 * /listen list
 * /listen clear
 */
import { Plugin } from "../../../core/plugin";
import type { MessageEvent } from "../../../core/types";
import { Command } from "../../../decorators/handlers";
import { RegisterPlugin } from "../../../decorators/register";
import { PermissionFilter } from "../../../filters/index";

@RegisterPlugin({
	name: "listen-manager",
	author: "gospeak",
	desc: "Manage SFU listen rooms: add/remove/list/clear",
	version: "1.0.0",
})
export class ListenManagerPlugin extends Plugin {
	@Command("listen", {
		desc: "Manage listen rooms",
		filters: [new PermissionFilter("moderator")],
	})
	async onListen(event: MessageEvent): Promise<void> {
		const sub = event.rawCommand?.args[0];
		const room = event.rawCommand?.args[1];
		const listen = this.ctx.listen;
		if (!listen) {
			await this.ctx.chat.reply(event, "listen capability is disabled");
			return;
		}

		switch (sub) {
			case "add": {
				if (!room) {
					await this.ctx.chat.reply(event, "usage: /listen add <room>");
					return;
				}
				const ok = listen.add(room);
				await this.ctx.chat.reply(
					event,
					ok ? `listening: ${room}` : `already listening: ${room}`,
				);
				return;
			}
			case "remove": {
				if (!room) {
					await this.ctx.chat.reply(event, "usage: /listen remove <room>");
					return;
				}
				const ok = listen.remove(room);
				await this.ctx.chat.reply(
					event,
					ok ? `stopped listening: ${room}` : `not listening: ${room}`,
				);
				return;
			}
			case "list": {
				const rooms = listen.list();
				await this.ctx.chat.reply(
					event,
					rooms.length
						? `listen rooms:\n${rooms.map((r) => `  - ${r}`).join("\n")}`
						: "no listen rooms",
				);
				return;
			}
			case "clear": {
				const removed = listen.clear();
				await this.ctx.chat.reply(
					event,
					removed.length
						? `cleared (${removed.length}): ${removed.join(", ")}`
						: "listen list already empty",
				);
				return;
			}
			default:
				await this.ctx.chat.reply(
					event,
					"usage: /listen add|remove|list|clear",
				);
		}
	}
}
