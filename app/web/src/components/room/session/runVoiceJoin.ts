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

// 同 room+identity 串行队列（不改 useVoiceSession）：
// token effect 重入会 abort+新 join；必须等旧 join 的 leave 收尾后再 WHIP，
// 否则第一次成功、第二次 stream busy。
const joinQueues = new Map<string, Promise<void>>();

function joinKey(token: JoinTokenResponse): string {
	return `${token.room}::${token.identity}::${token.stream || ""}`;
}

async function withJoinQueue<T>(
	key: string,
	fn: () => Promise<T>,
): Promise<T> {
	const prev = joinQueues.get(key) ?? Promise.resolve();
	let release!: () => void;
	const gate = new Promise<void>((resolve) => {
		release = resolve;
	});
	// 串起队列：后继等待本轮 gate 释放
	joinQueues.set(
		key,
		prev.catch(() => undefined).then(() => gate),
	);
	try {
		await prev.catch(() => undefined);
		return await fn();
	} finally {
		release();
		// 若仍是当前 gate 链尾，清掉避免 Map 无限增长
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

export async function runVoiceJoin(
	token: JoinTokenResponse,
	deps: VoiceJoinDeps,
): Promise<{ client: SFUClient; provider: SFUProvider }> {
	const provider = resolveSFUProvider(token);
	const adapter = getVoiceProviderAdapter(provider);
	const key = joinKey(token);

	// WHIP 类走同 key 串行；其他 provider 直接跑，避免误伤。
	if (adapter.interactiveAfterMedia) {
		return withJoinQueue(key, () => doVoiceJoin(token, deps, provider, adapter));
	}
	return doVoiceJoin(token, deps, provider, adapter);
}

async function doVoiceJoin(
	token: JoinTokenResponse,
	deps: VoiceJoinDeps,
	provider: SFUProvider,
	adapter: ReturnType<typeof getVoiceProviderAdapter>,
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
