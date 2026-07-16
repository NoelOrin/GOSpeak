/**
 * 语音反应插件：OnSpeechFinal 触发规则（唤醒词 / 违规词 / 语音命令）。
 */
import { Plugin } from "../../../core/plugin";
import type { SpeechEvent } from "../../../core/types";
import { EventType } from "../../../core/types";
import { On } from "../../../decorators/handlers";
import { RegisterPlugin } from "../../../decorators/register";

@RegisterPlugin({
	name: "voice-react",
	author: "gospeak",
	desc: "语音规则：唤醒词回复、违规词警告、语音踢人命令",
	version: "1.0.0",
})
export class VoiceReactPlugin extends Plugin {
	private lastAt = new Map<string, number>();

	private cooled(key: string, ms: number): boolean {
		const now = Date.now();
		const last = this.lastAt.get(key) ?? 0;
		if (now - last < ms) return false;
		this.lastAt.set(key, now);
		return true;
	}

	@On(EventType.OnSpeechFinal, { desc: "语音规则引擎" })
	async onSpeech(event: SpeechEvent): Promise<void> {
		const text = (event.text || "").trim();
		if (!text) return;
		const lower = text.toLowerCase();
		const cooldown = Number(this.ctx.config.cooldownMs ?? 10000);
		if (!this.cooled(`${event.room}:${event.speaker}`, cooldown)) return;

		const wake = String(this.ctx.config.wakeWord ?? "小助手").toLowerCase();
		if (lower.includes(wake)) {
			const reply = String(this.ctx.config.wakeReply ?? "我在，请说。");
			await this.ctx.chat.send(event.room, reply);
			if (this.ctx.voice.speak) {
				await this.ctx.voice.speak(event.room, reply);
			}
			return;
		}

		const banned: string[] = Array.isArray(this.ctx.config.bannedWords)
			? (this.ctx.config.bannedWords as string[])
			: [];
		for (const w of banned) {
			if (w && lower.includes(String(w).toLowerCase())) {
				await this.ctx.chat.send(event.room, `⚠️ ${event.speaker} 请注意用语`);
				return;
			}
		}

		const kickMatch = text.match(/踢出\s*(\S+)/) || text.match(/kick\s+(\S+)/i);
		if (kickMatch?.[1]) {
			const target = kickMatch[1];
			await this.ctx.voice.removeMember(event.room, target);
			await this.ctx.chat.send(event.room, `已踢出 ${target}`);
			if (this.ctx.voice.speak) {
				await this.ctx.voice.speak(event.room, `已踢出 ${target}`);
			}
		}
	}
}
