package com.gospeak.twa

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.os.Build
import android.os.IBinder
import android.util.Log

/**
 * 前台保活 Service。
 *
 * 当 WebView 检测到用户切后台时，通过 JavaScript 桥接调用
 * [BridgeInterface.startForegroundService]，此 Service 启动一个前台通知
 * （"GOSpeak 语音通话中…"），提升应用进程优先级，降低被系统杀死的概率。
 *
 * 用户回前台后，[BridgeInterface.stopForegroundService] 停止此 Service。
 */
class KeepaliveService : Service() {

    companion object {
        const val CHANNEL_ID = "gospeak_voice_keepalive"
        const val NOTIFICATION_ID = 1001
        const val ACTION_STOP = "com.gospeak.twa.STOP_KEEPALIVE"
        private const val TAG = "KeepaliveService"
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == ACTION_STOP) {
            stopSelf()
            return START_NOT_STICKY
        }

        // 每次 onStartCommand 都重建通知，确保 Service 被系统重启后通知仍在。
        try {
            val notification = buildNotification()
            startForeground(NOTIFICATION_ID, notification)
        } catch (e: Exception) {
            // Android 13+ 如果用户拒绝了 FOREGROUND_SERVICE_MICROPHONE 权限
            // 或通知权限，startForeground 会抛 ForegroundServiceStartNotAllowedException。
            // 降级处理：不崩溃，仅记录日志（前台保活失败，但 app 仍然可用）。
            Log.w(TAG, "startForeground failed, keepalive degraded: ${e.message}")
        }

        // 如果 Service 被杀，系统自动重启（带初始 intent）
        return START_REDELIVER_INTENT
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onDestroy() {
        stopForeground(STOP_FOREGROUND_REMOVE)
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            "GOSpeak 语音",
            NotificationManager.IMPORTANCE_LOW, // LOW 不弹出声音，只在状态栏显示
        ).apply {
            description = "GOSpeak 后台语音通话保活通知"
            setShowBadge(false)
        }
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(): Notification {
        val builder = Notification.Builder(this, CHANNEL_ID)
            .setContentTitle("GOSpeak")
            .setContentText("语音通话中…")
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setOngoing(true)  // 不可滑动清除
            .setPriority(Notification.PRIORITY_LOW)

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            builder.setForegroundServiceBehavior(
                Notification.FOREGROUND_SERVICE_IMMEDIATE
            )
        }

        return builder.build()
    }
}
