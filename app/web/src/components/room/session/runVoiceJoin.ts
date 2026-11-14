import type {
	SFUClient,
	SFUClientOptions,
	SFUProvider,
	SignalSocket,
} from "@gospeak/sfu-client/types";
import { type JoinTokenResponse, resolveSFUProvider } from "@/api/sfu";
import { toastActionError } from "@/utils/toast";
import { getVoiceProviderAdapter } from "./providers";
import type {
	VoiceJoinAck,
	VoicePhase,
	VoiceProviderAdapter,
} from "./voiceSessionTypes";

export class VoiceJoinAbortError extends Error {
	name = "AbortError";
}

export type VoiceJoinResult = {
	client: SFUClient;
	provider: SFUProvider;
};

/**
 * 单个 voice session 的 join 状态机片段：同目标 join 进行中时复用同一 promise。
 * 生命周期跟随 session（useVoiceSession 每实例持有一个），不存在跨 session 串扰，
 * 也无需模块级 Map 的手工 set/finally 清理；重入防护由 session transition
 * （abort 旧 join → setSession 新状态）保证。
 */
export interface VoiceJoinCoordinator {
	run(
		key: string,
		execute: () => Promise<VoiceJoinResult>,
	): Promise<VoiceJoinResult>;
}

export function createVoiceJoinCoordinator(): VoiceJoinCoordinator {
	const inFlight = new Map<string, Promise<VoiceJoinResult>>();
	return {
		run(key, execute) {
			const existing = inFlight.get(key);
			if (existing) return existing;
			const join = execute().finally(() => {
				if (inFlight.get(key) === join) {
					inFlight.delete(key);
				}
			});
			inFlight.set(key, join);
			return join;
		},
	};
}

export type VoiceJoinDeps = {
	loadClient: (
		provider: SFUProvider,
		options?: SFUClientOptions,
	) => Promise<SFUClient>;
	setupAudio: (client: SFUClient) => void;
	joinSignalRoom: (
		room: string,
		password?: string,
		domain_uuid?: string,
	) => Promise<unknown>;
	joinSignalSfu: (
		room: string,
		stream?: string,
		domain_uuid?: string,
	) => Promise<VoiceJoinAck>;
	connectSignal?: (workerUrl: string) => Promise<unknown>;
	/** 切换 Worker 前捕获当前房间，用于切流后恢复文字房信令槽位。 */
	getCurrentRoom?: () => {
		room: string;
		domain_uuid?: string;
		type?: string;
	} | null;
	rejoinTextRoom?: (
		room: string,
		domain_uuid: string | undefined,
	) => Promise<unknown>;
	onPhase: (phase: VoicePhase) => void;
	/**
	 * media join 成功后立刻回调（所有 provider 通用）。
	 * WHIP 类用此挂上 client，让 VoiceChat 在 media_ready 即可用，不必等信令结束。
	 */
	onClientReady?: (client: SFUClient, provider: SFUProvider) => void;
	audioOptions: SFUClientOptions;
	socket?: SignalSocket | null;
	password?: string;
	signal?: AbortSignal;
	/** session 状态机片段：serializeJoins=true 的 provider 依赖它做同目标 join 幂等 */
	coordinator?: VoiceJoinCoordinator;
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
): Promise<VoiceJoinResult> {
	const provider = resolveSFUProvider(token);
	const adapter = getVoiceProviderAdapter(provider);
	const run = () => executeVoiceJoin(token, deps, provider, adapter);

	if (adapter.serializeJoins) {
		const key = resolveJoinKey(adapter, token);
		if (deps.coordinator) {
			return deps.coordinator.run(key, run);
		}
		// 无 session 状态机时无法安全去重：直接执行，
		// 防重入由 session transition（abort 旧 join → 新状态）保证。
		return run();
	}
	return run();
}

async function executeVoiceJoin(
	token: JoinTokenResponse,
	deps: VoiceJoinDeps,
	provider: SFUProvider,
	adapter: VoiceProviderAdapter,
): Promise<VoiceJoinResult> {
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
	const previousRoom = deps.getCurrentRoom?.() ?? null;

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
				room: token.sfuRoom || token.room,
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
			if (token.workerUrl && deps.connectSignal) {
				await deps.connectSignal(token.workerUrl);
				if (
					previousRoom &&
					deps.rejoinTextRoom &&
					previousRoom.type === "text" &&
					(previousRoom.room !== token.room ||
						(previousRoom.domain_uuid || "") !== (token.domain_uuid || ""))
				) {
					await deps.rejoinTextRoom(
						previousRoom.room,
						previousRoom.domain_uuid,
					);
				}
			}
			await raceAbort(
				joinSignalRoom(token.room, password, token.domain_uuid),
				signal,
			);
			throwIfAborted(signal);

			const ack = await raceAbort(
				joinSignalSfu(token.room, token.stream, token.domain_uuid),
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
				.catch(async (err) => {
					if (
						signal?.aborted ||
						err instanceof VoiceJoinAbortError ||
						(err instanceof Error && err.name === "AbortError")
					) {
						return;
					}
					console.error("[runVoiceJoin] signal join failed after media:", err);
					// media 已 ready、外层 try/catch 已经返回，这里的失败原本只把
					// phase 置为 failed（error: null），用户看不到任何原因。
					toastActionError(err, "语音信令连接失败，请重新进入房间");
					onPhase("failed");
					await client.leaveRoom().catch(() => {});
					await client.destroy().catch(() => {});
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
