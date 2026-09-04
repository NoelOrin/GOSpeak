/**
 * SpeakingDetector — 本地麦克风音量判定的滞回状态机。
 *
 * 原始音量判定（avg > 阈值）逐帧抖动，直接上报会闪烁。Detector 要求新状态
 * 连续保持 holdOn/holdOff 毫秒后才翻转并上报，只返回状态发生变化的时刻，
 * 其余调用返回 null。用于 SRS / Cloudflare 等无 SFU 原生 active speaker 的
 * provider，配合信令层聚合 + 服务端去重，把发言态上报降到每次真实变化。
 */
export class SpeakingDetector {
	private current = false;
	// -1 表示无候选；不能用 0 做哨兵，因为采样时间戳可能恰好为 0。
	private candidateSince = -1;

	constructor(
		private readonly holdOnMs: number,
		private readonly holdOffMs: number,
	) {}

	/**
	 * 输入一帧原始判定，返回需要上报的新状态；状态未变化返回 null。
	 * 注意：holdOn/holdOff 需大于采样周期，否则退化为无滞回的直通。
	 */
	update(now: number, raw: boolean): boolean | null {
		if (raw === this.current) {
			this.candidateSince = -1;
			return null;
		}
		if (this.candidateSince === -1) {
			this.candidateSince = now;
			return null;
		}
		const required = raw ? this.holdOnMs : this.holdOffMs;
		if (now - this.candidateSince < required) {
			return null;
		}
		this.current = raw;
		this.candidateSince = -1;
		return raw;
	}

	/** 强制回到未发言状态（静音/停麦/拆流时立即通知），状态本已是 false 则返回 null。 */
	forceFalse(): boolean | null {
		if (!this.current) {
			this.candidateSince = -1;
			return null;
		}
		this.current = false;
		this.candidateSince = -1;
		return false;
	}

	reset(): void {
		this.current = false;
		this.candidateSince = -1;
	}

	get currentState(): boolean {
		return this.current;
	}
}
