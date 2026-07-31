import { BotRunner } from "./runtime/botRunner";

const SERVER_URL = process.env.GOSPEAK_SERVER_URL ?? "http://localhost:8998";
const SOCKET_URL = process.env.GOSPEAK_SOCKET_URL ?? SERVER_URL;
const ACCESS_TOKEN = process.env.GOSPEAK_TOKEN ?? "";
const BOT_USERNAME = process.env.GOSPEAK_BOT_USERNAME ?? "";
const BOT_PASSWORD = process.env.GOSPEAK_BOT_PASSWORD ?? "";
const BOT_IDENTITY = process.env.GOSPEAK_BOT_IDENTITY ?? "gospeak-bot";
const BOT_NAME = process.env.GOSPEAK_BOT_NAME ?? "GOSpeak Bot";
const AUTO_JOIN_ROOMS = process.env.GOSPEAK_AUTO_JOIN_ROOMS ?? "";
const LISTEN_ROOMS = process.env.GOSPEAK_LISTEN_ROOMS ?? "";
const PLUGIN_DIR = process.env.GOSPEAK_PLUGIN_DIR ?? "./plugins";
const ENABLE_SPEAK = process.env.GOSPEAK_ENABLE_SPEAK === "1";
const TTS_PROVIDER = process.env.GOSPEAK_TTS_PROVIDER ?? "edge";
const TTS_VOICE = process.env.GOSPEAK_TTS_VOICE ?? "";
const TTS_LANG = process.env.GOSPEAK_TTS_LANG ?? "";
const TTS_RATE = process.env.GOSPEAK_TTS_RATE ?? "";
const TTS_PITCH = process.env.GOSPEAK_TTS_PITCH ?? "";
const TTS_VOLUME = process.env.GOSPEAK_TTS_VOLUME ?? "";
const ASR_URL = process.env.GOSPEAK_ASR_URL ?? "";
const ASR_PROVIDER =
	process.env.GOSPEAK_ASR_PROVIDER ?? (ASR_URL ? "openai-compatible" : "none");
const ASR_API_KEY = process.env.GOSPEAK_ASR_API_KEY ?? "";
const ASR_MODEL = process.env.GOSPEAK_ASR_MODEL ?? "";
const ASR_LANGUAGE = process.env.GOSPEAK_ASR_LANGUAGE ?? "";

function parseNumber(v: string | undefined): number | undefined {
	if (!v) return undefined;
	const n = Number(v);
	return Number.isFinite(n) ? n : undefined;
}

function parseList(v: string): string[] | undefined {
	const items = v
		.split(",")
		.map((s) => s.trim())
		.filter(Boolean);
	return items.length ? items : undefined;
}

async function main(): Promise<void> {
	const hasToken = ACCESS_TOKEN.length > 0;
	const hasCredentials = BOT_USERNAME.length > 0 && BOT_PASSWORD.length > 0;

	if (!hasToken && !hasCredentials) {
		console.error(
			"Provide either GOSPEAK_TOKEN or GOSPEAK_BOT_USERNAME + GOSPEAK_BOT_PASSWORD",
		);
		process.exit(1);
	}

	const runner = new BotRunner({
		serverUrl: SERVER_URL,
		socketUrl: SOCKET_URL,
		accessToken: hasToken ? ACCESS_TOKEN : undefined,
		credentials: hasCredentials
			? { username: BOT_USERNAME, password: BOT_PASSWORD }
			: undefined,
		identity: BOT_IDENTITY,
		displayName: BOT_NAME,
		pluginDir: PLUGIN_DIR,
		autoJoinRooms: parseList(AUTO_JOIN_ROOMS),
		listenRooms: parseList(LISTEN_ROOMS),
		enableListen:
			Boolean(parseList(LISTEN_ROOMS)) ||
			process.env.GOSPEAK_ENABLE_LISTEN === "1",
		enableSpeak: ENABLE_SPEAK,
		tts: {
			provider: TTS_PROVIDER === "sine" ? "sine" : "edge",
			voice: TTS_VOICE || undefined,
			lang: TTS_LANG || undefined,
			rate: TTS_RATE || undefined,
			pitch: TTS_PITCH || undefined,
			volume: TTS_VOLUME || undefined,
		},
		asr: {
			provider:
				ASR_PROVIDER === "passthrough"
					? "passthrough"
					: ASR_URL
						? "openai-compatible"
						: "none",
			apiUrl: ASR_URL || undefined,
			apiKey: ASR_API_KEY || undefined,
			model: ASR_MODEL || undefined,
			language: ASR_LANGUAGE || undefined,
			minSilenceMs: parseNumber(process.env.GOSPEAK_ASR_MIN_SILENCE_MS),
			maxChunkMs: parseNumber(process.env.GOSPEAK_ASR_MAX_CHUNK_MS),
			minChunkMs: parseNumber(process.env.GOSPEAK_ASR_MIN_CHUNK_MS),
			vadThreshold: parseNumber(process.env.GOSPEAK_ASR_VAD_THRESHOLD),
		},
	});

	process.on("SIGINT", async () => {
		console.log("\nShutting down...");
		await runner.stop();
		process.exit(0);
	});

	process.on("SIGTERM", async () => {
		await runner.stop();
		process.exit(0);
	});

	await runner.start();

	setInterval(() => {
		const s = runner.status;
		console.log(
			`[bot] ${s.pluginCount} plugins, ${s.handlerCount} handlers, connected: ${s.connected}, auth: ${s.loggedIn}, listen: ${s.listenRooms.join(",") || "-"}`,
		);
	}, 30_000);

	await new Promise(() => {});
}

main().catch((err) => {
	console.error("Fatal error:", err);
	process.exit(1);
});
