declare module "socket.io-client" {
	export function connect(
		uri?: string,
		opts?: Partial<{
			transports: string[];
			[key: string]: any;
		}>,
	): Socket;
	export function io(
		uri?: string,
		opts?: Partial<{
			transports: string[];
			[key: string]: any;
		}>,
	): Socket;
	export default connect;

	class Socket {
		id: string | undefined;
		connected: boolean;
		disconnected: boolean;
		on(event: string, fn: (...args: any[]) => void): this;
		once(event: string, fn: (...args: any[]) => void): this;
		off(event?: string, fn?: (...args: any[]) => void): this;
		emit(event: string, ...args: any[]): this;
		connect(): this;
		open(): this;
		disconnect(): this;
		close(): this;
		compress(compress: boolean): this;
		send(...args: any[]): this;
		write(...args: any[]): this;
	}
}
