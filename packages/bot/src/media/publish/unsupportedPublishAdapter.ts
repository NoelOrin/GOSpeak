import type { SFUPublishAdapter } from "./types";

/** Explicit failure adapter for SFU providers without a Node publish implementation yet. */
export class UnsupportedPublishAdapter implements SFUPublishAdapter {
	constructor(public readonly provider: string) {}

	async join(): Promise<void> {
		throw new Error(
			`SFU publish adapter not implemented for provider: ${this.provider}`,
		);
	}

	async publishPcm(): Promise<void> {
		throw new Error(
			`SFU publish adapter not implemented for provider: ${this.provider}`,
		);
	}

	async unpublish(): Promise<void> {
		throw new Error(
			`SFU publish adapter not implemented for provider: ${this.provider}`,
		);
	}

	async leave(): Promise<void> {
		throw new Error(
			`SFU publish adapter not implemented for provider: ${this.provider}`,
		);
	}

	async dispose(): Promise<void> {}
}
