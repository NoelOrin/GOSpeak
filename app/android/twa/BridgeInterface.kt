package com.gospeak.twa

import android.content.Intent
import android.webkit.JavascriptInterface

/**
 * JavaScript ↔ Android 原生桥接接口。
 *
 * 通过 WebView.addJavascriptInterface 注入到页面，
 * 对应前端 [KeepaliveAdapter] 的 twaBridge 选项。
 *
 * 使用示例（WebView 侧）：
 * ```kotlin
 * webView.addJavascriptInterface(BridgeInterface(this), "GOSpeakBridge")
 * ```
 *
 * 前端通过 `window.GOSpeakBridge.startForegroundService()` 调用。
 */
class BridgeInterface(
    private val appContext: android.content.Context,
) {
    /**
     * 启动前台保活 Service。
     * 前端切后台时调用，触发常驻通知，提升进程优先级。
     */
    @JavascriptInterface
    fun startForegroundService() {
        val intent = Intent(appContext, KeepaliveService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            appContext.startForegroundService(intent)
        } else {
            appContext.startService(intent)
        }
    }

    /**
     * 停止前台保活 Service。
     * 前端回前台时调用，移除通知，释放资源。
     */
    @JavascriptInterface
    fun stopForegroundService() {
        val intent = Intent(appContext, KeepaliveService::class.java)
        appContext.stopService(intent)
    }

    /**
     * 获取当前设备/应用信息。
     * 可供前端判断是否处于 TWA 环境。
     */
    @JavascriptInterface
    fun getPlatformInfo(): String = "android-twa"
}
