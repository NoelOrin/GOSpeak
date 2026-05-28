declare module "socket.io-client" {
  interface Socket {
    id: string | undefined;
    connected: boolean;
    io: { engine: { transport: { name: string } } };
    on(event: string, callback: (...args: any[]) => void): Socket;
    emit(event: string, ...args: any[]): Socket;
    disconnect(): Socket;
  }

  function io(uri: string, opts?: Record<string, any>): Socket;
  export default io;
  export type { Socket };
}
