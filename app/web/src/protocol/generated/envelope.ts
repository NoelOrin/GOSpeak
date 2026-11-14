// GENERATED — 手改无效，跑 pnpm gen:protocol
// ACK 信封约定（与 ws.ACK 对应）：成功 data 携带结果、失败走 error 通道。
// emitAck 成功路径 resolve 的是 data 本身（非信封）。
export interface AckFrame<T = unknown> {
	id?: string;
	event: string;
	data?: T;
	error?: { code: number; message: string };
}

// AckPayload<T>：emitAck 成功路径 resolve 的 data 类型。
// P1 各事件 data 尚未全部类型化（PayloadOf 于 P2 补齐），统一先 unknown。
export type AckPayload<T> = T;

export type AckResult = AckPayload<unknown>;
