/**
 * Platform and capability detection for keepalive strategies.
 */

export interface KeepaliveOptions {
	/** PiP / 保活状态变化回调 */
	onStateChange?: (active: boolean) => void;
	/** 心跳回调，每 30s 触发一次，可用于检测保活连接健康度 */
	onHeartbeat?: (elapsedMs: number) => void;
	/**
	 * 纯音频房间的自定义 Canvas 绘制回调。
	 * 不传时使用默认样式（深色背景 + GOSpeak logo）。
	 */
	onCanvasRender?: (
		ctx: CanvasRenderingContext2D,
		canvas: HTMLCanvasElement,
	) => void;
	/** 切后台时自动进入保活模式（默认 true） */
	autoEnterOnBackground?: boolean;
	/** 回前台时自动退出保活模式（默认 true） */
	autoExitOnForeground?: boolean;

	// --- Canvas fallback ---
	canvasWidth?: number;
	canvasHeight?: number;

	// --- Android ---
	/**
	 * Android 原生桥接对象。
	 *
	 * TWA 包装中通过 WebView.addJavascriptInterface(name, "GOSpeakBridge")
	 * 注入的对象，前端通过 window.GOSpeakBridge 访问。
	 *
	 * 方法签名：
	 *   startForegroundService(): void  — 启动前台 Service + 常驻通知
	 *   stopForegroundService(): void   — 停止前台 Service
	 *
	 * 不传时 Android 浏览器/PWA 场景仅尝试 WakeLock（不可靠）。
	 */
	twaBridge?: TWAInterface | null;

	/** 调试日志 */
	debug?: boolean;
}

/** Android 原生桥接接口（对应 BridgeInterface.kt） */
export interface TWAInterface {
	startForegroundService(): void;
	stopForegroundService(): void;
}

// ================================================================
//  Platform detection
// ================================================================

export enum Platform {
	IOS = "ios",
	Android = "android",
	AndroidTWA = "android-twa",
	Other = "other",
}

export function detectPlatform(twaBridge?: TWAInterface | null): Platform {
	const ua = navigator.userAgent;
	const isIOS =
		/iPad|iPhone|iPod/.test(ua) ||
		(navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
	const isAndroid = /Android/.test(ua);

	if (isIOS) return Platform.IOS;
	if (isAndroid && twaBridge) return Platform.AndroidTWA;
	if (isAndroid) return Platform.Android;
	return Platform.Other;
}

// ================================================================
//  Feature detection (static)
// ================================================================

export function checkPiPSupport(): boolean {
	try {
		const v = document.createElement("video");
		return (
			"pictureInPictureEnabled" in document &&
			document.pictureInPictureEnabled &&
			("webkitSupportsPresentationMode" in v || "requestPictureInPicture" in v)
		);
	} catch {
		return false;
	}
}

export function checkWakeLockSupport(): boolean {
	return (
		"wakeLock" in navigator && typeof navigator.wakeLock?.request === "function"
	);
}
