package com.gospeak.twa

import android.os.Bundle
import android.webkit.WebChromeClient
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.appcompat.app.AppCompatActivity

/**
 * TWA 主 Activity 示例。
 *
 * 演示如何配置 WebView + 注入 BridgeInterface，
 * 使前端 KeepaliveAdapter 能通过 twaBridge 调用原生保活。
 */
class MainActivity : AppCompatActivity() {

    private lateinit var webView: WebView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        webView = WebView(this)
        setContentView(webView)

        configureWebView()
        loadGOSpeak()
    }

    private fun configureWebView() {
        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            databaseEnabled = true
            useWideViewPort = true
            loadWithOverviewMode = true
            allowFileAccess = false
            mediaPlaybackRequiresUserGesture = false  // 允许自动播放音频

            // WebRTC 支持
            mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
            setSupportZoom(false)
            builtInZoomControls = false
        }

        webView.webChromeClient = WebChromeClient()
        webView.webViewClient = WebViewClient()

        // 注入原生桥接 — 前端通过 window.GOSpeakBridge.* 访问
        webView.addJavascriptInterface(
            BridgeInterface(applicationContext),
            "GOSpeakBridge",
        )
    }

    private fun loadGOSpeak() {
        // 替换为你的 GOSpeak 部署地址
        webView.loadUrl("https://your-gospeak-domain.com")
    }

    override fun onBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack()
        } else {
            super.onBackPressed()
        }
    }
}
