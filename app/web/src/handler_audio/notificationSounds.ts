let ctx: AudioContext | null = null;
let enabled = true;

function getCtx(): AudioContext {
	if (!ctx) ctx = new AudioContext();
	return ctx;
}

export function setNotificationSoundEnabled(v: boolean) {
	enabled = v;
}

export function isNotificationSoundEnabled(): boolean {
	return enabled;
}

function playTone(
	freq: number,
	duration: number,
	type: OscillatorType,
	ramp: number,
) {
	if (!enabled) return;
	const c = getCtx();
	if (c.state === "suspended") c.resume();
	const osc = c.createOscillator();
	const gain = c.createGain();
	osc.type = type;
	osc.frequency.setValueAtTime(freq, c.currentTime);
	gain.gain.setValueAtTime(0.3, c.currentTime);
	gain.gain.exponentialRampToValueAtTime(0.001, c.currentTime + ramp);
	osc.connect(gain);
	gain.connect(c.destination);
	osc.start(c.currentTime);
	osc.stop(c.currentTime + duration);
}

/** 加入房间提示音：短促上行音 */
export function playJoinSound() {
	if (!enabled) return;
	const c = getCtx();
	if (c.state === "suspended") c.resume();
	// 快速两个上行音
	playTone(523, 0.1, "sine", 0.08);
	setTimeout(() => playTone(659, 0.15, "sine", 0.12), 80);
}

/** 离开房间提示音：短促下行音 */
export function playLeaveSound() {
	if (!enabled) return;
	const c = getCtx();
	if (c.state === "suspended") c.resume();
	playTone(440, 0.12, "triangle", 0.1);
	setTimeout(() => playTone(349, 0.12, "triangle", 0.1), 100);
}
