export interface TTSProvider {
	readonly name: string;
	synthesize(text: string): Promise<Int16Array>;
}
