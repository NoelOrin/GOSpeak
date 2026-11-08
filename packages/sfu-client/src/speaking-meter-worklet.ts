/**
 * AudioWorklet 处理器源码（以字符串内联，经 Blob URL 加载）。
 *
 * 运行在音频渲染线程，作为硬件节拍的事件源：每 reportBlocks 个音频块
 * （128 帧/块）向主线程 postMessage 一次；主线程在消息回调里读取 AnalyserNode
 * 完成判定（保持既有阈值/滞回语义）。音频透传到输出，主线程侧用 0 增益节点
 * 静音路由，避免回声。
 *
 * 用字符串内联而非独立文件：new URL(..., import.meta.url) 的资源发射对
 * workspace 链接包不可靠（Vite 不会重写该调用，生产构建下 worklet 会 404），
 * Blob URL 方案与打包器无关，dev/build 行为一致。
 */
export const SPEAKING_METER_WORKLET_SOURCE = `
const DEFAULT_REPORT_BLOCKS = 16;

class SpeakingMeterProcessor extends AudioWorkletProcessor {
	constructor(options) {
		super();
		const opts = (options && options.processorOptions) || {};
		this.reportBlocks = Math.max(1, opts.reportBlocks || DEFAULT_REPORT_BLOCKS);
		this.blockCount = 0;
	}

	process(inputs, outputs) {
		const input = inputs[0];
		const output = outputs[0];
		if (input && output && input[0] && output[0]) {
			output[0].set(input[0]);
		}
		this.blockCount++;
		if (this.blockCount >= this.reportBlocks) {
			this.blockCount = 0;
			this.port.postMessage({});
		}
		return true;
	}
}

registerProcessor("gospeak-speaking-meter", SpeakingMeterProcessor);
`;
