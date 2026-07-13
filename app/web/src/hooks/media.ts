export interface AudioDeviceInfo {
	deviceId: string;
	groupId: string;
	label: string;
	kind: "audioinput" | "audiooutput";
}

export async function getAudioDevices(): Promise<{
	audioinputs: AudioDeviceInfo[];
	audiooutputs: AudioDeviceInfo[];
}> {
	try {
		const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
		for (const t of stream.getTracks()) {
			t.stop();
		}
	} catch {
		// permission denied or no mic
	}

	const devices = await navigator.mediaDevices.enumerateDevices();
	return {
		audioinputs: devices
			.filter((d) => d.kind === "audioinput")
			.map((d) => ({
				deviceId: d.deviceId,
				groupId: d.groupId,
				label: d.label,
				kind: d.kind as "audioinput",
			})),
		audiooutputs: devices
			.filter((d) => d.kind === "audiooutput")
			.map((d) => ({
				deviceId: d.deviceId,
				groupId: d.groupId,
				label: d.label,
				kind: d.kind as "audiooutput",
			})),
	};
}
