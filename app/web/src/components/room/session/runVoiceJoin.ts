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
import type {
	VoiceJoinAck,
	VoicePhase,
	VoiceProviderAdapter,
} from "./voiceSessionTypes";

export class VoiceJoinAbortError extends Error {
	name = "AbortError";
}

// adapter.serializeJoins=true 时按 joinKey 串行（SRS 防双 WHIP）。
// LiveKit 等 serializeJoins=false，互不影响。
const joinQueues = new Map<string, Promise<void>>();

async function withJoinQueue<T>(
	key: string,
	fn: () => Promise<T>,
): Promise<T> {
	const prev = joinQueues.get(key) ?? Promise.resolve();
	let release!: () => void;
	const gate = new Promise<void>((resolve) => {
		release = resolve;
	});
	joinQueues.set(
		key,
		prev.catch(() => undefined).then(() => gate),
	);
	try {
		await prev.catch(() => undefined);
		return await fn();
	} finally {
		release();
		const cur = joinQueues.get(key);
		void cur?.then(() => {
			if (joinQueues.get(key) === cur) {
				joinQueues.delete(key);
			}
		});
	}
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

function resolveJoinKey(
	adapter: VoiceProviderAdapter,
	token: JoinTokenResponse,
): string {
	return (
		adapter.joinKey?.(token) ??
		`${token.room}::${token.identity}::${token.stream || ""}`
	);
}

/**
 * 通用进房编排。provider 差异只读 adapter：
 * - resolveConnectTarget
 * - serializeJoins / joinKey
 * - interactiveAfterMedia / signalJoinMode
 * - afterMediaJoin
 *
 * 禁止在此写 provider 名称分支。
 */
export async function runVoiceJoin(
	token: JoinTokenResponse,
	deps: VoiceJoinDeps,
): Promise<{ client: SFUClient; provider: SFUProvider }> {
	const provider = resolveSFUProvider(token);
	const adapter = getVoiceProviderAdapter(provider);
	const run = () => executeVoiceJoin(token, deps, provider, adapter);

	if (adapter.serializeJoins) {
		return withJoinQueue(resolveJoinKey(adapter, token), run);
	}
	return run();
}

async function executeVoiceJoin(
	token: JoinTokenResponse,
	deps: VoiceJoinDeps,
	provider: SFUProvider,
	adapter: VoiceProviderAdapter,
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

	onPhase("loading_sfu");
	const client = await raceAbort(
		loadClient(provider, {
			...audioOptions,
			socket,
		}),
		signal,
	);
	throwIfAborted(signal);

	// abort 时拆掉已创建 client，避免媒体会话孤儿。
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

		// media 已通：先挂 client，再切 phase。
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

		const signalMode = adapter.signalJoinMode ?? "await";
		const earlyInteractive =
			!!adapter.interactiveAfterMedia || signalMode === "background";

		// background：media 成功即 media_ready 并返回；信令后台继续。
		if (earlyInteractive) {
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

		// await：完整 signal 后才返回（LiveKit 等）。
		onPhase("joining_signal");
		await joinSignal();
		return { client, provider };
	} catch (err) {
		await client.leaveRoom().catch(() => {});
		await client.destroy().catch(() => {});
		throw err;
	}
}
