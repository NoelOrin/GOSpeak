export type ScreenShareQuality = "original" | "1080p" | "720p" | "480p";

export interface ScreenShareQualityPreset {
	label: string;
	/**
	 * 目标发送分辨率高度；0 表示不缩放，按采集到的原始尺寸发送。
	 *
	 * 档位通过编码侧缩放（sender encodings 的 scaleResolutionDownBy）实现，
	 * 不重新采集：这样共享中改档是即时的、可逆的，也不会再次弹出窗口选择框。
	 */
	maxHeight: number;
	encoding: { maxBitrate: number; maxFramerate: number };
	contentHint: "detail" | "motion";
}

/**
 * 屏幕共享发送端档位。
 *
 * 只影响本地编码分辨率与发送码率，不参与接收端选层：
 * 订阅侧由 LiveKit 的 adaptiveStream / dynacast 自行决定，再叠一层手动档位只会互相打架。
 */
export const SCREEN_SHARE_QUALITY_PRESETS = {
	original: {
		label: "原画",
		maxHeight: 0,
		encoding: { maxBitrate: 7_000_000, maxFramerate: 30 },
		contentHint: "detail",
	},
	"1080p": {
		label: "1080p",
		maxHeight: 1080,
		encoding: { maxBitrate: 4_000_000, maxFramerate: 30 },
		contentHint: "detail",
	},
	"720p": {
		label: "720p",
		maxHeight: 720,
		encoding: { maxBitrate: 2_000_000, maxFramerate: 30 },
		contentHint: "detail",
	},
	"480p": {
		label: "480p",
		maxHeight: 480,
		encoding: { maxBitrate: 800_000, maxFramerate: 15 },
		contentHint: "motion",
	},
} satisfies Record<ScreenShareQuality, ScreenShareQualityPreset>;

/** UI 展示顺序：从清晰到省流。 */
export const SCREEN_SHARE_QUALITY_ORDER: ScreenShareQuality[] = [
	"original",
	"1080p",
	"720p",
	"480p",
];

export const DEFAULT_SCREEN_SHARE_QUALITY: ScreenShareQuality = "1080p";

const KNOWN_QUALITIES = new Set<string>(SCREEN_SHARE_QUALITY_ORDER);

/** 持久化值可能来自旧版本或被改写，未知档位一律回落到默认值。 */
export function normalizeScreenShareQuality(
	value: unknown,
): ScreenShareQuality {
	return typeof value === "string" && KNOWN_QUALITIES.has(value)
		? (value as ScreenShareQuality)
		: DEFAULT_SCREEN_SHARE_QUALITY;
}

export function screenShareQualityPreset(
	quality: ScreenShareQuality,
): ScreenShareQualityPreset {
	return (
		SCREEN_SHARE_QUALITY_PRESETS[quality] ??
		SCREEN_SHARE_QUALITY_PRESETS[DEFAULT_SCREEN_SHARE_QUALITY]
	);
}

/** 档位标签，未知档位回落到默认档标签。 */
export function screenShareQualityLabel(quality: ScreenShareQuality): string {
	return screenShareQualityPreset(quality).label;
}

export interface ScreenShareCaptureResolution {
	width: number;
	height: number;
	frameRate: number;
}

/**
 * 采集始终按原始分辨率进行（0 宽高 = 不限制），档位只在编码侧下采样。
 *
 * 按档位限制采集会锁死上限：先降到 480p 再切回原画时，采集源已经丢了细节，
 * 编码器再怎么放大也回不去。统一按原尺寸采集、按档位缩放编码，两个方向的切换
 * 才是对称的，也才能做到共享中改档既不重新采集、也不再弹一次窗口选择框。
 */
export const SCREEN_SHARE_CAPTURE_RESOLUTION: ScreenShareCaptureResolution = {
	width: 0,
	height: 0,
	frameRate: 30,
};

/**
 * 计算顶层编码的 scaleResolutionDownBy。
 *
 * 采集按原始尺寸进行，档位只缩放编码输出，因此捕获分辨率低于目标时不需要放大。
 */
export function screenShareScaleDown(
	capturedHeight: number | undefined,
	maxHeight: number,
): number {
	if (!capturedHeight || capturedHeight <= 0) return 1;
	if (maxHeight <= 0) return 1;
	if (capturedHeight <= maxHeight) return 1;
	return capturedHeight / maxHeight;
}

export interface ScreenShareLayerEncoding {
	scaleResolutionDownBy: number;
	maxBitrate: number;
	maxFramerate: number;
}

/** 最低层码率下限：再低画面就没法看了，LiveKit 的默认分层也用同一个下限。 */
const MIN_LAYER_BITRATE = 150_000;

/**
 * 按档位重算 simulcast 各层编码参数。
 *
 * 层数由发布时决定且必须保持不变（rid 顺序是 LiveKit 识别分层的依据），
 * 因此这里只改每层的缩放比与码率：顶层对齐档位目标高度，低层按 2 的幂次
 * 逐级缩小分辨率、按 4 的幂次逐级降低码率。
 */
export function computeScreenShareLayerEncodings(
	layerCount: number,
	quality: ScreenShareQuality,
	capturedHeight?: number,
): ScreenShareLayerEncoding[] {
	const preset = screenShareQualityPreset(quality);
	const topScale = screenShareScaleDown(capturedHeight, preset.maxHeight);
	const count = Math.max(1, Math.floor(layerCount));
	const encodings: ScreenShareLayerEncoding[] = [];
	for (let index = 0; index < count; index += 1) {
		const stepsBelowTop = count - 1 - index;
		const scale = topScale * 2 ** stepsBelowTop;
		const bitrate = Math.max(
			MIN_LAYER_BITRATE,
			Math.floor(preset.encoding.maxBitrate / 4 ** stepsBelowTop),
		);
		encodings.push({
			scaleResolutionDownBy: scale,
			maxBitrate: bitrate,
			maxFramerate: preset.encoding.maxFramerate,
		});
	}
	return encodings;
}
