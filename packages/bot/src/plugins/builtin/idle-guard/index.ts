/**
 * 空闲巡检插件：定时扫 joined 房间，默认仅警告。
 */
import { Plugin } from "../../../core/plugin";
import { RegisterPlugin } from "../../../decorators/register";

@RegisterPlugin({
	name: "idle-guard",
	author: "gospeak",
	desc: "定时巡检房间空闲状态（默认 warnOnly）",
	version: "1.0.0",
})
export class IdleGuardPlugin extends Plugin {
	async onLoad(): Promise<void> {
		const everyMs = Number(this.ctx.config.everyMs ?? 30000);
		this.ctx.scheduler.every("scan", everyMs, async () => {
			const rooms = this.ctx.rooms.joined();
			for (const room of rooms) {
				try {
					const members = await this.ctx.rooms.getMembers(room);
					if (members.length <= 1) {
						// only bot or empty-ish
						continue;
					}
					// mild strategy: log only unless configured
					this.ctx.logger.debug(
						`[idle-guard] room ${room} members=${members.length}`,
					);
					if (this.ctx.config.warnOnly !== false) {
					}
				} catch (err) {
					this.ctx.logger.warn(`[idle-guard] scan failed for ${room}`, err);
				}
			}
		});
	}

	async onUnload(): Promise<void> {
		this.ctx.scheduler.clearAll();
	}
}
