import { DeepgramASRProvider } from "./deepgramProvider";
import { LocalHttpASRProvider } from "./localHttpProvider";
import type { ASRProvider } from "./types";

export function createASRProvider(
	env: NodeJS.ProcessEnv = process.env,
): ASRProvider {
	const name = (env.GOSPEAK_ASR_PROVIDER || "local").toLowerCase();
	switch (name) {
		case "deepgram":
			return new DeepgramASRProvider({
				apiKey: env.GOSPEAK_ASR_DEEPGRAM_KEY,
			});
		case "local":
		default:
			return new LocalHttpASRProvider({
				url: env.GOSPEAK_ASR_URL,
			});
	}
}
