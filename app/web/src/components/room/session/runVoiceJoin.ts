import type {
	SFUClient,
	SFUClientOptions,
	SFUProvider,
} from "@gospeak/sfu-client/types";
import {
	type JoinTokenResponse,
	resolveSFUProvider,
} from "@/api/sfu";
import { getVoiceProviderAdapter } from "./providers";
import type { VoiceJoinAck, VoicePhase } from "./voiceSessionTypes";

export class VoiceJoinAbortError extends Error {
	name = "AbortError";
}

export type VoiceJoinDeps = {
	loadClient: (
		provider: SFUProvider,
		options?: SFUClientOptions,
	) => Promise<SFUClient>;
	setupAudio: (client: SFUClient) => void;
	joinSignalRoom: (
		room: string,
		identity: string,
		password?: string,
	) => Promise<unknown>;
	joinSignalSfu: (
		room: string,
		identity: string,
		stream?: string,
	) => Promise<VoiceJoinAck>;
	onPhase: (phase: VoicePhase) => void;
	audioOptions: SFUClientOptions;
	socket?: unknown;
	password?: string;
	signal?: AbortSignal;
};

function throwIfAborted(signal?: AbortSignal): void {
	if (signal?.aborted) {
		throw new VoiceJoinAbortError();
	}
}

function raceAbort<T>(p: Promise<T>, signal?: AbortSignal): Promise<T> {
	if (!signal) return p;
	if (signal.aborted) return Promise.reject(new VoiceJoinAbortError());
	return new Promise<T>((resolve, reject) => {
		const onAbort = () => reject(new VoiceJoinAbortError());
		signal.addEventListener("abort", onAbort, { once: true });
		p.then(
			(v) => {
				signal.removeEventListener("abort", onAbort);
				resolve(v);
			},
			(e) => {
				signal.removeEventListener("abort", onAbort);
				reject(e);
			},
		);
	});
}

export async function runVoiceJoin(
	token: JoinTokenResponse,
	deps: VoiceJoinDeps,
): Promise<{ client: SFUClient; provider: SFUProvider }> {
	const {
		loadClient,
		setupAudio,
		joinSignalRoom,
		joinSignalSfu,
		onPhase,
		audioOptions,
		socket,
		password,
		signal,
	} = deps;

	throwIfAborted(signal);

	const provider = resolveSFUProvider(token);
	const adapter = getVoiceProviderAdapter(provider);

	onPhase("loading_sfu");
	const client = await raceAbort(
		loadClient(provider, {
			...audioOptions,
			socket,
		}),
		signal,
	);
	throwIfAborted(signal);

	onPhase("joining_media");
	await raceAbort(
		client.joinRoom({
			token: token.token,
			serverUrl: adapter.resolveConnectTarget(token),
			identity: token.identity,
			room: token.room,
			stream: token.stream,
			streamToken: token.streamToken,
		}),
		signal,
	);
	throwIfAborted(signal);

	setupAudio(client);

	onPhase("joining_signal");
	await raceAbort(
		joinSignalRoom(token.room, token.identity, password),
		signal,
	);
	throwIfAborted(signal);

	const ack = await raceAbort(
		joinSignalSfu(token.room, token.identity, token.stream),
		signal,
	);
	throwIfAborted(signal);

	await adapter.afterMediaJoin?.(client, token, ack ?? {});

	return { client, provider };
}
