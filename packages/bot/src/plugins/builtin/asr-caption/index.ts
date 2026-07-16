/**
 * ASR 字幕插件：订阅 OnSpeechFinal，默认只 log；broadcast=true 时发 bot:message。
 */
import { Plugin } from "../../../core/plugin";
import type { SpeechEvent } from "../../../core/types";
import { EventType } from "../../../core/types";
import { On } from "../../../decorators/handlers";
import { RegisterPlugin } from "../../../decorators/register";

@RegisterPlugin({
	name: "asr-caption",
	author: "gospeak",
	desc: "语音转文字字幕：OnSpeechFinal → log / 可选广播",
	version: "1.0.0",
})
export class AsrCaptionPlugin extends Plugin {
	private lastAt = new Map<string, number>();

	@On(EventType.OnSpeechFinal, { desc: "字幕输出" })
	async onFinal(event: SpeechEvent): Promise<void> {
		const text = event.text?.trim();
		if (!text) return;

		const key = `${event.room}:${event.speaker}`;
		const cooldownMs = Number(this.ctx.config.cooldownMs ?? 3000);
		const now = Date.now();
		const last = this.lastAt.get(key) ?? 0;
		if (now - last < cooldownMs) return;
		this.lastAt.set(key, now);

		this.ctx.logger.info(`[caption] ${event.room}/${event.speaker}: ${text}`);

		if (this.ctx.config.broadcast === true) {
			await this.ctx.chat.send(event.room, `🎙 ${event.speaker}: ${text}`);
		}
	}
}
