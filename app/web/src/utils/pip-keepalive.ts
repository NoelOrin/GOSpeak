/**
 * KeepaliveAdapter
 *
 * 移动端后台语音保活统一适配层。
 *
 * ┌──────────────────────┬────────────────────────────────────────┐
 * │ 平台                  │ 保活策略                                │
 * ├──────────────────────┼────────────────────────────────────────┤
 * │ iOS (Safari/WKWebView)│ PiP 画中画保持渲染进程活跃                │
 * │ Android (TWA)        │ 原生前台 Service 通知 + WakeLock 辅助    │
 * │ Android (浏览器/PWA)  │ WakeLock 仅（浏览器无保活能力）           │
 * └──────────────────────┴────────────────────────────────────────┘
 *
 * 纯抽象，不依赖任何 WebRTC SDK / 框架。
 * 输入：任意 MediaStream；输出：保活行为。
 */

// ================================================================
//  Types
// ================================================================

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

function detectPlatform(twaBridge?: TWAInterface | null): Platform {
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

function _checkPiPSupport(): boolean {
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

function _checkWakeLockSupport(): boolean {
	return (
		"wakeLock" in navigator && typeof navigator.wakeLock?.request === "function"
	);
}

// ================================================================
//  Internal state
// ================================================================

enum State {
	Idle,
	Entering,
	Active,
	Destroyed,
}

// ================================================================
//  Main class
// ================================================================

export class KeepaliveAdapter {
	// --- 只读平台信息 ---
	readonly platform: Platform;
	static readonly isPiPSupported = _checkPiPSupport();
	static readonly isWakeLockSupported = _checkWakeLockSupport();

	// --- 内部 ---
	private _state: State = State.Idle;
	private _opts: Required<KeepaliveOptions>;

	// iOS PiP 相关
	private _videoEl: HTMLVideoElement | null = null;

	// Canvas fallback
	private _canvasEl: HTMLCanvasElement | null = null;
	private _canvasTimer: ReturnType<typeof setInterval> | null = null;
	private _canvasStream: MediaStream | null = null;

	// Android 保活相关
	private _wakeLockSentinel: WakeLockSentinel | null = null;
	private _heartbeatTimer: ReturnType<typeof setInterval> | null = null;
	private _heartbeatStartedAt: number = 0;

	// 事件处理器（保存引用以便移除）
	private _boundHandlers: {
		enterpip: EventListener;
		leavepip: EventListener;
		pagehide: EventListener;
		pageshow: EventListener;
		visibility: EventListener;
	};

	constructor(opts: KeepaliveOptions = {}) {
		this.platform = detectPlatform(opts.twaBridge ?? null);

		this._opts = {
			onStateChange: opts.onStateChange ?? (() => {}),
			onHeartbeat: opts.onHeartbeat ?? (() => {}),
			onCanvasRender: opts.onCanvasRender ?? this._defaultCanvasRender,
			autoEnterOnBackground: opts.autoEnterOnBackground ?? true,
			autoExitOnForeground: opts.autoExitOnForeground ?? true,
			canvasWidth: opts.canvasWidth ?? 320,
			canvasHeight: opts.canvasHeight ?? 180,
			twaBridge: opts.twaBridge ?? null,
			debug: opts.debug ?? false,
		};

		this._boundHandlers = {
			enterpip: this._onEnterPiP.bind(this),
			leavepip: this._onLeavePiP.bind(this),
			pagehide: this._onPageHide.bind(this),
			pageshow: this._onPageShow.bind(this),
			visibility: this._onVisibilityChange.bind(this),
		};

		this._log("platform:", this.platform);
		this._log(
			"supports:",
			`PiP=${KeepaliveAdapter.isPiPSupported}`,
			`WakeLock=${KeepaliveAdapter.isWakeLockSupported}`,
		);

		if (this.platform === Platform.Android && !opts.twaBridge) {
			this._log(
				"Android browser/PWA detected — no twaBridge provided.",
				"Background keepalive will NOT work reliably.",
			);
		}
	}

	// ================================================================
	//  Public API
	// ================================================================

	/** 当前是否处于保活活跃状态 */
	get isActive(): boolean {
		return this._state === State.Active;
	}

	// ---- 流管理 ----

	/**
	 * 绑定一个 MediaStream（来自任意 WebRTC SDK）。
	 *
	 * iOS PiP 需要 video 元素来 feed PiP 窗口；
	 * Android 不需要流，但仍保留方法以保持 API 一致。
	 */
	attachStream(stream: MediaStream): void {
		if (this._state === State.Destroyed) return;

		if (this.platform === Platform.IOS) {
			this._ensureVideoElement();
			this._videoEl!.srcObject = stream;
			this._videoEl!.play().catch(() => {});
		}

		this._log("stream attached");
	}

	/** 解绑当前流 */
	detachStream(): void {
		if (this._videoEl) {
			this._videoEl.srcObject = null;
		}
	}

	// ---- 保活控制 ----

	/**
	 * 进入保活模式。
	 *
	 * - iOS：进入 PiP（需要用户手势触发第一次）
	 * - Android TWA：启动前台 Service + WakeLock
	 * - Android 浏览器：仅尝试 WakeLock（不可靠）
	 */
	async enter(): Promise<boolean> {
		if (this._state === State.Destroyed) return false;
		if (this._state === State.Active) return true;

		this._state = State.Entering;

		switch (this.platform) {
			case Platform.IOS:
				return this._enterPiP();
			case Platform.AndroidTWA:
				return this._enterAndroidTWA();
			case Platform.Android:
				return this._enterAndroidFallback();
			default:
				this._state = State.Idle;
				return false;
		}
	}

	/**
	 * 退出保活模式。
	 */
	async exit(): Promise<boolean> {
		if (this._state === State.Destroyed) return true;
		if (this._state !== State.Active) return true;

		switch (this.platform) {
			case Platform.IOS:
				return this._exitPiP();
			case Platform.AndroidTWA:
				return this._exitAndroidKeepalive();
			case Platform.Android:
				return this._exitAndroidKeepalive();
			default:
				return true;
		}
	}

	/**
	 * 彻底销毁，释放所有资源。
	 */
	destroy(): void {
		if (this._state === State.Destroyed) return;

		this.exit().catch(() => {});
		this._stopHeartbeat();
		this._cleanupCanvas();
		this._detachDOMEvents();
		this._removeVideoElement();

		this._state = State.Destroyed;
		this._log("destroyed");
	}

	// ---- 音频房间 Canvas 流 ----

	/**
	 * 为纯音频房间创建一个 Canvas fallback 流。
	 *
	 * 房间无视频轨道时调用此方法生成一个静态画面流供 PiP 使用。
	 *
	 * @param displayText Canvas 上显示的文字（例如房间名）。不传则只显示默认 GOSpeak logo。
	 * @returns Canvas MediaStream，iOS 上自动喂给 PiP video 元素
	 */
	createCanvasFallbackStream(displayText?: string): MediaStream {
		this._cleanupCanvas();

		const w = this._opts.canvasWidth;
		const h = this._opts.canvasHeight;
		const canvas = document.createElement("canvas");
		canvas.width = w;
		canvas.height = h;
		canvas.style.position = "fixed";
		canvas.style.left = "-9999px";
		canvas.style.top = "-9999px";
		canvas.style.width = "1px";
		canvas.style.height = "1px";
		canvas.setAttribute("aria-hidden", "true");
		this._canvasEl = canvas;

		const ctx = canvas.getContext("2d")!;
		const render = () => {
			this._opts.onCanvasRender(ctx, canvas);
			if (displayText) {
				ctx.fillStyle = "#ffffff";
				ctx.font = "16px sans-serif";
				ctx.textAlign = "center";
				ctx.fillText(displayText, w / 2, h - 20);
			}
		};
		render();
		this._canvasTimer = setInterval(render, 1000);

		const stream = canvas.captureStream(1);
		this._canvasStream = stream;

		// iOS 上自动喂给 video 元素供 PiP 使用
		if (this._videoEl) {
			this._videoEl.srcObject = stream;
			this._videoEl.play().catch(() => {});
		}

		return stream;
	}

	/** 停止 Canvas fallback 流 */
	stopCanvasFallback(): void {
		this._cleanupCanvas();
	}

	// ================================================================
	//  iOS PiP
	// ================================================================

	private _ensureVideoElement(): HTMLVideoElement {
		if (this._videoEl) return this._videoEl;

		const video = document.createElement("video");
		video.muted = true;
		video.playsInline = true;
		video.loop = false;
		video.style.position = "fixed";
		video.style.left = "-9999px";
		video.style.top = "-9999px";
		video.style.width = "1px";
		video.style.height = "1px";
		video.setAttribute("aria-hidden", "true");
		document.body.appendChild(video);

		this._videoEl = video;
		this._attachDOMEvents(video);
		return video;
	}

	private _removeVideoElement(): void {
		if (!this._videoEl) return;
		this._detachDOMEvents();
		this._videoEl.srcObject = null;
		this._videoEl.remove();
		this._videoEl = null;
	}

	private async _enterPiP(): Promise<boolean> {
		if (!KeepaliveAdapter.isPiPSupported) {
			this._log("PiP not supported on this device");
			this._state = State.Idle;
			return false;
		}

		const video = this._ensureVideoElement();
		if (!video.srcObject) {
			this.createCanvasFallbackStream();
		}

		try {
			if (video.requestPictureInPicture) {
				await video.requestPictureInPicture();
			} else if ((video as any).webkitSetPresentationMode) {
				await (video as any).webkitSetPresentationMode("picture-in-picture");
			}
			// State 由 enterpip 事件处理
			return true;
		} catch (err) {
			this._state = State.Idle;
			this._log("enterPiP failed:", err);
			return false;
		}
	}

	private async _exitPiP(): Promise<boolean> {
		try {
			if (document.pictureInPictureElement) {
				await document.exitPictureInPicture();
			} else if ((document as any).webkitPictureInPictureElement) {
				await (document as any).webkitExitPictureInPicture();
			}
			return true;
		} catch (err) {
			this._log("exitPiP failed:", err);
			return false;
		}
	}

	// ================================================================
	//  Android TWA 保活
	// ================================================================

	/**
	 * Android TWA 保活：
	 *
	 * 1. 原生前台 Service 通知 — 主策略，提升进程优先级
	 *    对应 app/android/twa/KeepaliveService.kt
	 *
	 * 2. WakeLock — 辅助，阻止 CPU 深度休眠
	 *    需要 AndroidManifest 中声明 WAKE_LOCK 权限
	 *
	 * 3. Heartbeat — 检测保活连接健康度
	 *
	 * ⚠️ 前置条件：
	 *   - WebView 需注入 GOSpeakBridge（见 BridgeInterface.kt）
	 *   - AndroidManifest 需声明 FOREGROUND_SERVICE + 前台 Service
	 */
	private async _enterAndroidTWA(): Promise<boolean> {
		let anyActive = false;
		const promises: Promise<unknown>[] = [];

		this._log("entering Android TWA keepalive");

		// 1. 原生前台 Service（主策略）
		if (this._opts.twaBridge) {
			try {
				this._opts.twaBridge.startForegroundService();
				anyActive = true;
				this._log("TWA foreground service started");
			} catch (err) {
				this._log("TWA bridge startForegroundService failed:", err);
			}
		}

		// 2. WakeLock（辅助）
		if (KeepaliveAdapter.isWakeLockSupported) {
			promises.push(
				this._acquireWakeLock().then((ok) => {
					if (ok) anyActive = true;
				}),
			);
		}

		// 3. Heartbeat
		this._startHeartbeat();

		await Promise.allSettled(promises);

		if (anyActive) {
			this._state = State.Active;
			this._opts.onStateChange(true);
			return true;
		}

		this._log("no Android keepalive strategy succeeded");
		this._state = State.Idle;
		return false;
	}

	/**
	 * Android 纯浏览器/PWA 保活（不可靠）。
	 *
	 * 仅尝试 WakeLock，无原生桥接。
	 * 浏览器后台冻结是系统行为，前端无法阻止。
	 * 建议切换回前台时做快速重连，而不是试图保活。
	 */
	private async _enterAndroidFallback(): Promise<boolean> {
		this._log("Android browser fallback — keepalive NOT guaranteed");

		if (KeepaliveAdapter.isWakeLockSupported) {
			const ok = await this._acquireWakeLock();
			if (ok) {
				this._state = State.Active;
				this._opts.onStateChange(true);
				return true;
			}
		}

		// 无 WakeLock → 无法保活，但不阻塞业务流程
		this._log("WakeLock unavailable, keepalive impossible on this device");
		this._state = State.Idle;
		return false;
	}

	private async _exitAndroidKeepalive(): Promise<boolean> {
		// 1. 停止前台 Service
		if (this._opts.twaBridge) {
			try {
				this._opts.twaBridge.stopForegroundService();
				this._log("TWA foreground service stopped");
			} catch (err) {
				this._log("TWA bridge stop failed:", err);
			}
		}

		// 2. Release WakeLock
		await this._releaseWakeLock();

		// 3. Stop Heartbeat
		this._stopHeartbeat();

		this._state = State.Idle;
		this._opts.onStateChange(false);
		return true;
	}

	// ---- WakeLock ----

	private async _acquireWakeLock(): Promise<boolean> {
		try {
			this._wakeLockSentinel = await (navigator.wakeLock as any).request(
				"system",
			);
			this._wakeLockSentinel!.addEventListener("release", () => {
				this._log("WakeLock released externally");
				this._wakeLockSentinel = null;
			});
			this._log("WakeLock acquired");
			return true;
		} catch (err) {
			this._log("WakeLock acquisition failed:", err);
			return false;
		}
	}

	private async _releaseWakeLock(): Promise<void> {
		if (this._wakeLockSentinel) {
			try {
				await this._wakeLockSentinel.release();
			} catch {
				// ignore
			}
			this._wakeLockSentinel = null;
		}
	}

	// ---- Heartbeat ----

	private _startHeartbeat(): void {
		this._stopHeartbeat();
		this._heartbeatStartedAt = Date.now();

		const timer = setInterval(() => {
			if (this._state === State.Destroyed) {
				clearInterval(timer);
				return;
			}
			this._opts.onHeartbeat(Date.now() - this._heartbeatStartedAt);
		}, 30_000);
		this._heartbeatTimer = timer;
	}

	private _stopHeartbeat(): void {
		if (this._heartbeatTimer) {
			clearInterval(this._heartbeatTimer);
			this._heartbeatTimer = null;
		}
	}

	// ================================================================
	//  DOM 事件
	// ================================================================

	private _attachDOMEvents(video: HTMLVideoElement): void {
		video.addEventListener(
			"enterpictureinpicture",
			this._boundHandlers.enterpip,
		);
		video.addEventListener(
			"leavepictureinpicture",
			this._boundHandlers.leavepip,
		);
		window.addEventListener("pagehide", this._boundHandlers.pagehide);
		window.addEventListener("pageshow", this._boundHandlers.pageshow);
		document.addEventListener(
			"visibilitychange",
			this._boundHandlers.visibility,
		);
	}

	private _detachDOMEvents(): void {
		if (this._videoEl) {
			this._videoEl.removeEventListener(
				"enterpictureinpicture",
				this._boundHandlers.enterpip,
			);
			this._videoEl.removeEventListener(
				"leavepictureinpicture",
				this._boundHandlers.leavepip,
			);
		}
		window.removeEventListener("pagehide", this._boundHandlers.pagehide);
		window.removeEventListener("pageshow", this._boundHandlers.pageshow);
		document.removeEventListener(
			"visibilitychange",
			this._boundHandlers.visibility,
		);
	}

	// ---- 状态事件 ----

	private _onEnterPiP(): void {
		const wasInactive = this._state !== State.Active;
		this._state = State.Active;
		if (wasInactive) this._opts.onStateChange(true);
		this._log("PiP entered");
	}

	private _onLeavePiP(): void {
		const wasActive = this._state === State.Active;
		this._state = State.Idle;
		if (wasActive) this._opts.onStateChange(false);
		this._log("PiP left");
	}

	// ---- 页面可见性自动控制 ----

	private _onPageHide(): void {
		if (this._opts.autoEnterOnBackground && this.platform === Platform.IOS) {
			this.enter();
		}
	}

	private _onPageShow(): void {
		if (this._opts.autoExitOnForeground) {
			this.exit();
		}
	}

	private _onVisibilityChange(): void {
		if (document.visibilityState === "hidden") {
			if (this._opts.autoEnterOnBackground && this.platform === Platform.IOS) {
				this.enter();
			}
		}
		if (document.visibilityState === "visible") {
			if (this._opts.autoExitOnForeground) {
				this.exit();
			}
		}
	}

	// ================================================================
	//  Canvas 管理
	// ================================================================

	private _cleanupCanvas(): void {
		if (this._canvasTimer) {
			clearInterval(this._canvasTimer);
			this._canvasTimer = null;
		}
		if (this._canvasStream) {
			this._canvasStream.getTracks().forEach((t) => t.stop());
			this._canvasStream = null;
		}
		if (this._canvasEl) {
			this._canvasEl.remove();
			this._canvasEl = null;
		}
	}

	// ================================================================
	//  默认 Canvas 绘制
	// ================================================================

	private _defaultCanvasRender = (
		ctx: CanvasRenderingContext2D,
		canvas: HTMLCanvasElement,
	): void => {
		const w = canvas.width;
		const h = canvas.height;

		ctx.fillStyle = "#1a1a2e";
		ctx.fillRect(0, 0, w, h);

		ctx.fillStyle = "#4ade80";
		ctx.beginPath();
		ctx.arc(w / 2, h / 2 - 10, 20, 0, Math.PI * 2);
		ctx.fill();

		ctx.fillStyle = "#ffffff";
		ctx.font = "bold 14px sans-serif";
		ctx.textAlign = "center";
		ctx.fillText("GOSpeak", w / 2, h - 12);
	};

	// ================================================================
	//  Logging
	// ================================================================

	private _log(...args: unknown[]): void {
		if (this._opts.debug) {
			console.log(`[KeepaliveAdapter:${this.platform}]`, ...args);
		}
	}
}
