import { Plugin } from "../../core/plugin";
import { RegisterPlugin } from "../../decorators/register";
import { Command } from "../../decorators/handlers";
import { PermissionFilter, RegexFilter } from "../../filters/index";
import { EventType } from "../../core/types";
import type { MessageEvent } from "../../core/types";

@RegisterPlugin({
	name: "echo",
	author: "gospeak",
	desc: "A reference plugin demonstrating command and filter handlers.",
	version: "0.1.0",
})
export class EchoPlugin extends Plugin {
	@Command("echo", { alias: ["say"], desc: "Echo back the message text." })
	async onEcho(event: MessageEvent): Promise<void> {
		const text = event.rawCommand?.args.join(" ") ?? "";
		await this.ctx.chat.reply(event, `echo: ${text}`);
	}

	@Command("hello", { filters: [new RegexFilter(/hi/i)], desc: "Reply to a greeting." })
	async onHello(event: MessageEvent): Promise<void> {
		await this.ctx.chat.reply(event, "hi there!");
	}

	@Command("kick", {
		prefix: "/",
		desc: "Kick a member (admin only).",
		filters: [new PermissionFilter("admin")],
	})
	async onKick(event: MessageEvent): Promise<void> {
		const target = event.rawCommand?.args[0];
		if (target) {
			await this.ctx.voice.removeMember(event.room.id, target);
			await this.ctx.chat.reply(event, `kicked ${target}`);
		}
	}
}
