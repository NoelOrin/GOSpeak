declare module "socket.io-client" {
	export interface Socket {
		on(event: string, fn: (...args: any[]) => void): this;
		emit(event: string, ...args: any[]): this;
		disconnect(): this;
		connect(): this;
		readonly connected: boolean;
		id?: string;
	}
	export function io(url: string, opts?: Record<string, unknown>): Socket;
	export { Socket as default };
}
