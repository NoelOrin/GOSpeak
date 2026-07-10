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
	/**
	 * media join 成功后立刻回调（所有 provider 通用）。
	 * WHIP 类用此挂上 client，让 VoiceChat 在 media_ready 即可用，不必等信令结束。
	 */
	onClientReady?: (client: SFUClient, provider: SFUProvider) => void;
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
		onClientReady,
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

	// 切房 abort 时必须拆掉已创建的 client，否则 SRS WHIP/WHEP 会孤儿悬挂。
	try {
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

		// media 已通：先挂 client，再切 phase。WHIP 类进 media_ready 即可渲染 VoiceChat。
		onClientReady?.(client, provider);
		throwIfAborted(signal);

		const joinSignal = async () => {
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
		};

		// WHIP/WHEP：media 成功即 media_ready 并立刻返回，VoiceChat 可加载。
		// 信令/成员订阅后台继续；失败不得拆已成功的 media session。
		if (adapter.interactiveAfterMedia) {
			onPhase("media_ready");
			void joinSignal()
				.then(() => {
					if (signal?.aborted) return;
					onPhase("ready");
				})
				.catch((err) => {
					if (
						signal?.aborted ||
						err instanceof VoiceJoinAbortError ||
						(err instanceof Error && err.name === "AbortError")
					) {
						return;
					}
					console.error("[runVoiceJoin] signal join failed after media:", err);
				});
			return { client, provider };
		}

		onPhase("joining_signal");
		await joinSignal();
		return { client, provider };
	} catch (err) {
		await client.leaveRoom().catch(() => {});
		await client.destroy().catch(() => {});
		throw err;
	}
}
