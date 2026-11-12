/**
 * Browser-side media probes for GOSpeak voice rooms.
 * Injected via page.evaluate / page.addInitScript helpers.
 */

export const MEDIA_PROBE_SOURCE = String.raw`
(() => {
  if (window.__gospeakMediaProbe) return window.__gospeakMediaProbe;

  const state = {
    getUserMediaCalls: 0,
    lastGetUserMediaAt: 0,
    localTracks: [],
    peerConnections: [],
    remoteAudioElements: [],
  };

  const originalGUM = navigator.mediaDevices?.getUserMedia?.bind(navigator.mediaDevices);
  if (originalGUM) {
    navigator.mediaDevices.getUserMedia = async (constraints) => {
      state.getUserMediaCalls += 1;
      state.lastGetUserMediaAt = Date.now();
      const stream = await originalGUM(constraints);
      for (const track of stream.getTracks()) {
        state.localTracks.push({
          id: track.id,
          kind: track.kind,
          label: track.label,
          enabled: track.enabled,
          muted: track.muted,
          readyState: track.readyState,
        });
      }
      return stream;
    };
  }

  const OriginalPC = window.RTCPeerConnection;
  if (OriginalPC) {
    window.RTCPeerConnection = class extends OriginalPC {
      constructor(...args) {
        super(...args);
        const entry = {
          id: state.peerConnections.length + 1,
          createdAt: Date.now(),
          connectionState: this.connectionState,
          iceConnectionState: this.iceConnectionState,
          signalingState: this.signalingState,
          senders: 0,
          receivers: 0,
        };
        state.peerConnections.push(entry);
        const refresh = () => {
          entry.connectionState = this.connectionState;
          entry.iceConnectionState = this.iceConnectionState;
          entry.signalingState = this.signalingState;
          try {
            entry.senders = this.getSenders().filter((s) => !!s.track).length;
            entry.receivers = this.getReceivers().filter((r) => !!r.track).length;
          } catch {
            // ignore transient closed PC access
          }
        };
        this.addEventListener("connectionstatechange", refresh);
        this.addEventListener("iceconnectionstatechange", refresh);
        this.addEventListener("track", refresh);
        refresh();
      }
    };
    window.RTCPeerConnection.prototype = OriginalPC.prototype;
  }

  function snapshotRemoteAudio() {
    const nodes = Array.from(document.querySelectorAll("audio, video"));
    return nodes
      .filter((el) => {
        const stream = el.srcObject;
        return !!stream && typeof stream.getAudioTracks === "function";
      })
      .map((el, index) => {
        const stream = el.srcObject;
        const tracks = stream.getAudioTracks();
        return {
          index,
          tag: el.tagName.toLowerCase(),
          paused: el.paused,
          muted: el.muted,
          volume: el.volume,
          currentTime: el.currentTime,
          readyState: el.readyState,
          trackCount: tracks.length,
          liveTracks: tracks.filter((t) => t.readyState === "live").length,
          enabledTracks: tracks.filter((t) => t.enabled).length,
        };
      });
  }

  async function collectRtcStats() {
    const pcs = [];
    // Best-effort: inspect known PC instances via RTCPeerConnection wrappers only.
    // Detailed getStats requires holding original instances; we expose connection states instead.
    for (const pc of state.peerConnections) {
      pcs.push({ ...pc });
    }
    return pcs;
  }

  window.__gospeakMediaProbe = {
    getSnapshot() {
      return {
        getUserMediaCalls: state.getUserMediaCalls,
        lastGetUserMediaAt: state.lastGetUserMediaAt,
        localTracks: state.localTracks.slice(-8),
        peerConnections: state.peerConnections.map((pc) => ({ ...pc })),
        remoteAudio: snapshotRemoteAudio(),
        currentRoomText: document.querySelector(".font-bold.truncate")?.textContent?.trim() || "",
        phaseText:
          Array.from(document.querySelectorAll("div, span"))
            .map((el) => el.textContent?.trim() || "")
            .find((t) =>
              [
                "准备加入...",
                "加载语音引擎...",
                "连接媒体...",
                "媒体已连接",
                "加入房间...",
                "正在重连...",
                "正在离开...",
                "加入失败",
                "已连接",
                "已连接，等待成员加入",
              ].includes(t),
            ) || "",
        memberCountText:
          Array.from(document.querySelectorAll("span"))
            .map((el) => el.textContent?.trim() || "")
            .find((t) => /\d+\s*人在线/.test(t)) || "",
        hasLeaveButton: Array.from(document.querySelectorAll("button")).some(
          (b) => b.textContent?.trim() === "离开",
        ),
        hasRetryButton: Array.from(document.querySelectorAll("button")).some(
          (b) => b.textContent?.trim() === "重试",
        ),
      };
    },
    async waitForMediaReady(timeoutMs = 20000) {
      const start = Date.now();
      while (Date.now() - start < timeoutMs) {
        const snap = window.__gospeakMediaProbe.getSnapshot();
        const hasJoinedUi = snap.hasLeaveButton || /人在线/.test(snap.memberCountText);
        const hasPc = snap.peerConnections.some((pc) =>
          ["connected", "completed", "checking", "connecting"].includes(pc.iceConnectionState) ||
          ["connected", "connecting"].includes(pc.connectionState),
        );
        const hasLocal = snap.localTracks.some((t) => t.kind === "audio" && t.readyState === "live");
        if (hasJoinedUi && (hasPc || hasLocal || snap.getUserMediaCalls > 0)) {
          return { ok: true, snapshot: snap };
        }
        if (snap.hasRetryButton || snap.phaseText === "加入失败") {
          return { ok: false, snapshot: snap, reason: "join_failed" };
        }
        await new Promise((r) => setTimeout(r, 250));
      }
      return {
        ok: false,
        snapshot: window.__gospeakMediaProbe.getSnapshot(),
        reason: "timeout",
      };
    },
    async waitForRemoteAudio(minTracks = 1, timeoutMs = 20000) {
      const start = Date.now();
      while (Date.now() - start < timeoutMs) {
        const remote = snapshotRemoteAudio().filter((a) => a.liveTracks > 0);
        if (remote.length >= minTracks) {
          return { ok: true, remote, snapshot: window.__gospeakMediaProbe.getSnapshot() };
        }
        await new Promise((r) => setTimeout(r, 250));
      }
      return {
        ok: false,
        remote: snapshotRemoteAudio(),
        snapshot: window.__gospeakMediaProbe.getSnapshot(),
        reason: "timeout",
      };
    },
    collectRtcStats,
  };

  return window.__gospeakMediaProbe;
})();
`;

export async function installMediaProbe(page) {
  await page.addInitScript(MEDIA_PROBE_SOURCE);
  await page.evaluate(MEDIA_PROBE_SOURCE);
}

export async function getMediaSnapshot(page) {
  return page.evaluate(() => window.__gospeakMediaProbe?.getSnapshot?.() || null);
}

export async function waitForMediaReady(page, timeoutMs = 20000) {
  return page.evaluate(
    async (timeout) => window.__gospeakMediaProbe.waitForMediaReady(timeout),
    timeoutMs,
  );
}

export async function waitForRemoteAudio(page, minTracks = 1, timeoutMs = 20000) {
  return page.evaluate(
    async ({ minTracks: min, timeoutMs: timeout }) =>
      window.__gospeakMediaProbe.waitForRemoteAudio(min, timeout),
    { minTracks, timeoutMs },
  );
}
