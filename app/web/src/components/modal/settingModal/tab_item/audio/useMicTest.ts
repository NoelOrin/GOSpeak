import { toast } from "@gospeak/components";
import { useEffect, useRef, useState } from "react";

export function useMicTest({
	deviceId,
	echoCancellation,
	noiseSuppression,
	autoGainControl,
}: {
	deviceId: string;
	echoCancellation: boolean;
	noiseSuppression: boolean;
	autoGainControl: boolean;
}) {
	const [testing, setTesting] = useState(false);
	const [level, setLevel] = useState(0);
	const testStreamRef = useRef<MediaStream | null>(null);
	const analyserRef = useRef<AnalyserNode | null>(null);
	const audioCtxRef = useRef<AudioContext | null>(null);
	const rafRef = useRef(0);

	function stopMicTest() {
		if (rafRef.current) cancelAnimationFrame(rafRef.current);
		rafRef.current = 0;
		testStreamRef.current?.getTracks().forEach((t) => {
			t.stop();
		});
		testStreamRef.current = null;
		void audioCtxRef.current?.close();
		audioCtxRef.current = null;
		analyserRef.current = null;
		setLevel(0);
		setTesting(false);
	}

	async function startMicTest() {
		if (testing) {
			stopMicTest();
			return;
		}
		try {
			const constraints: MediaStreamConstraints = {
				audio: {
					deviceId: deviceId ? { exact: deviceId } : undefined,
					echoCancellation,
					noiseSuppression,
					autoGainControl,
				},
				video: false,
			};
			testStreamRef.current =
				await navigator.mediaDevices.getUserMedia(constraints);
			audioCtxRef.current = new AudioContext();
			const source = audioCtxRef.current.createMediaStreamSource(
				testStreamRef.current,
			);
			analyserRef.current = audioCtxRef.current.createAnalyser();
			analyserRef.current.fftSize = 256;
			source.connect(analyserRef.current);
			const data = new Uint8Array(analyserRef.current.frequencyBinCount);
			const tick = () => {
				const analyser = analyserRef.current;
				if (!analyser) return;
				analyser.getByteTimeDomainData(data);
				let sum = 0;
				for (let i = 0; i < data.length; i++) {
					const v = (data[i] - 128) / 128;
					sum += v * v;
				}
				const rms = Math.sqrt(sum / data.length);
				setLevel(Math.min(100, Math.round(rms * 220)));
				rafRef.current = requestAnimationFrame(tick);
			};
			setTesting(true);
			tick();
		} catch (error) {
			toast.error(
				error instanceof Error && error.name === "NotAllowedError"
					? "麦克风权限被拒绝，请在浏览器设置中允许后重试"
					: "麦克风测试失败，请检查设备是否可用",
			);
			stopMicTest();
		}
	}

	// biome-ignore lint/correctness/useExhaustiveDependencies: 挂载/卸载时启停麦克风测试；stopMicTest 稳定，加入依赖会导致重复启停与设备抢占
	useEffect(() => {
		return () => {
			stopMicTest();
		};
	}, []);

	return { testing, level, toggle: startMicTest };
}
