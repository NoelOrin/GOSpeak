import { SpeakingDetector } from "./speaking-detector";
import { SPEAKING_METER_WORKLET_SOURCE } from "./speaking-meter-worklet";

export type LocalSpeakingMeterOptions = {
	/** AnalyserNode 频域均值阈值（保持既有判定语义，默认 10）。 */
	threshold?: number;
	holdOnMs?: number;
	holdOffMs?: number;
	/** 工作台每 reportBlocks 个音频块（128 帧）上报一次采样事件；16 ≈ 43ms @48kHz。 */
	reportBlocks?: number;
	onSpeakingChange?: (speaking: boolean) => void;
};

const WORKLET_NAME = "gospeak-speaking-meter";

/**
 * LocalSpeakingMeter — 本地麦克风音量检测，事件驱动、无 JS 定时器轮询。
 *
 * 采样时钟由 AudioWorkletProcessor 提供：它挂在音频渲染线程上，每处理完
 * reportBlocks 个音频块就 postMessage 一次；主线程在消息回调里读取
 * AnalyserNode 的频域数据（阈值语义与既有实现一致），经 SpeakingDetector
 * 滞回后仅在状态翻转时通知调用方。主线程侧用 0 增益节点静音路由，避免回声。
 */
export class LocalSpeakingMeter {
	private readonly detector: SpeakingDetector;
	private readonly threshold: number;
	private readonly reportBlocks: number;

	private context: AudioContext | null = null;
	private analyser: AnalyserNode | null = null;
	private clockNode: AudioWorkletNode | null = null;
	private levels: Uint8Array | null = null;
	private active = false;
	private boundStream: MediaStream | null = null;
	// 递增代数：并发 start/stop 竞态下，过期的异步 addModule 不得回写状态。
	private initToken = 0;

	constructor(private readonly options: LocalSpeakingMeterOptions = {}) {
		this.detector = new SpeakingDetector(
			options.holdOnMs ?? 120,
			options.holdOffMs ?? 300,
		);
		this.threshold = options.threshold ?? 10;
		this.reportBlocks = options.reportBlocks ?? 16;
	}

	get isActive(): boolean {
		return this.active;
	}

	/** 开始分析本地流；已在分析同一流时为 no-op，换流则先拆再起。 */
	start(stream: MediaStream): void {
		if (!stream) return;
		if (this.active && this.boundStream === stream) return;
		if (this.active) this.teardown();
		this.boundStream = stream;
		const token = ++this.initToken;
		void this.init(stream, token);
	}

	private async init(stream: MediaStream, token: number): Promise<void> {
		let ctx: AudioContext | null = null;
		try {
			ctx = new AudioContext();
			if (!ctx.audioWorklet || typeof ctx.audioWorklet.addModule !== "function") {
				console.warn("[speaking-meter] AudioWorklet unavailable, speaking detection disabled");
				void ctx.close().catch(() => {});
				return;
			}
			// Blob URL 内联加载：与打包器无关（new URL 资源发射对 workspace 链接包不可靠）。
			const workletUrl = URL.createObjectURL(
				new Blob([SPEAKING_METER_WORKLET_SOURCE], {
					type: "application/javascript",
				}),
			);
			try {
				await ctx.audioWorklet.addModule(workletUrl);
			} finally {
				URL.revokeObjectURL(workletUrl);
			}
			if (token !== this.initToken || this.boundStream !== stream) {
				void ctx.close().catch(() => {});
				return;
			}

			const source = ctx.createMediaStreamSource(stream);
			const analyser = ctx.createAnalyser();
			analyser.fftSize = 256;
			const silentAnalyser = ctx.createGain();
			silentAnalyser.gain.value = 0;
			const clockNode = new AudioWorkletNode(ctx, WORKLET_NAME, {
				numberOfInputs: 1,
				numberOfOutputs: 1,
				processorOptions: { reportBlocks: this.reportBlocks },
			});
			const silentClock = ctx.createGain();
			silentClock.gain.value = 0;
			source.connect(analyser);
			analyser.connect(silentAnalyser);
			silentAnalyser.connect(ctx.destination);
			clockNode.connect(silentClock);
			silentClock.connect(ctx.destination);

			const levels = new Uint8Array(analyser.frequencyBinCount);
			clockNode.port.onmessage = () => {
				if (!this.active) return;
				analyser.getByteFrequencyData(levels);
				let sum = 0;
				for (let i = 0; i < levels.length; i++) {
					sum += levels[i];
				}
				const change = this.detector.update(
					performance.now(),
					sum / levels.length > this.threshold,
				);
				if (change !== null) this.options.onSpeakingChange?.(change);
			};

			this.context = ctx;
			this.analyser = analyser;
			this.clockNode = clockNode;
			this.levels = levels;
			this.active = true;
		} catch (err) {
			console.warn("[speaking-meter] AudioWorklet init failed, speaking detection disabled", err);
			void ctx?.close().catch(() => {});
			// 仅当仍指向本次流时清空，避免误伤并发 start() 的新绑定。
			if (this.boundStream === stream) this.boundStream = null;
		}
	}

	/** 立即上报停麦并拆除分析（离开/停麦/拆流）。幂等。 */
	stop(): void {
		if (!this.active) {
			// init 可能仍在 addModule 途中：作废该次初始化，避免 stop 后 meter 复活。
			this.initToken++;
			this.boundStream = null;
			return;
		}
		this.forceFalse();
		this.teardown();
	}

	/** 仅上报停麦，不拆除（静音但本地流仍在，如 track.enabled=false）。幂等。 */
	forceFalse(): void {
		const change = this.detector.forceFalse();
		if (change !== null) this.options.onSpeakingChange?.(change);
	}

	private teardown(): void {
		this.initToken++;
		this.active = false;
		if (this.clockNode) {
			try {
				this.clockNode.port.close();
			} catch {
				// ignore
			}
			try {
				this.clockNode.disconnect();
			} catch {
				// ignore
			}
		}
		try {
			this.analyser?.disconnect();
		} catch {
			// ignore
		}
		this.detector.reset();
		void this.context?.close().catch(() => {});
		this.context = null;
		this.analyser = null;
		this.clockNode = null;
		this.levels = null;
		this.boundStream = null;
	}
}
