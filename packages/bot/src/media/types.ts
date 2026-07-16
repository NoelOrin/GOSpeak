/** 旁听媒体层统一音频帧（16-bit little-endian PCM） */
export interface AudioFrameEvent {
	room: string;
	identity: string;
	/** s16le PCM samples */
	pcm16: Int16Array;
	sampleRate: number;
	channels: number;
	timestamp: number;
	mediaProvider: string;
}

export interface PcmStreamFilter {
	room?: string;
	identity?: string;
}

/**
 * 进程内 PCM 可读流接口。
 * 外部 ASR / 录制 / 插件通过此接口读取旁听音频帧，不经 HTTP。
 */
export interface PcmStream {
	/** 订阅帧；返回取消函数 */
	subscribe(listener: (frame: AudioFrameEvent) => void): () => void;
	/** 按房间/说话人过滤订阅 */
	subscribeFiltered(
		filter: PcmStreamFilter,
		listener: (frame: AudioFrameEvent) => void,
	): () => void;
	/** 打开一个可关闭的读取会话（可 async iterate） */
	open(filter?: PcmStreamFilter): PcmStreamReader;
	readonly stats: {
		listeners: number;
		frameCount: number;
		lastFrameAt: number;
	};
}

/**
 * 单次读取会话：支持回调与 async iterator 两种消费方式。
 * close() 后停止接收。
 */
export interface PcmStreamReader extends AsyncIterable<AudioFrameEvent> {
	onFrame(listener: (frame: AudioFrameEvent) => void): () => void;
	close(): void;
	readonly closed: boolean;
}

/** 旁听层写入 PCM 的接口 */
export interface PcmStreamSink {
	publish(frame: AudioFrameEvent): void;
}
