import { bridge, command } from "../bridge";
import { isDesktop } from "../runtime";

export interface OpenDialogOptions {
	title?: string;
	/** e.g. "Audio files":["wav","mp3"] */
	filters?: Record<string, string[]>;
	multiple?: boolean;
}

export interface SaveDialogOptions {
	title?: string;
	defaultPath?: string;
	filters?: Record<string, string[]>;
}

export interface MessageBoxOptions {
	title?: string;
	message: string;
	kind?: "info" | "warning" | "error" | "question";
	buttons?: string[];
}

/** Native file/system dialogs. Falls back to <input type=file> / window.confirm in browser. */
export function openFileDialog(options?: OpenDialogOptions): Promise<string[]> {
	if (isDesktop()) {
		return bridge.invoke<string[]>(command("Dialog", "Open"), options);
	}
	return new Promise((resolve) => {
		const input = document.createElement("input");
		input.type = "file";
		input.multiple = options?.multiple ?? false;
		input.onchange = () => {
			const files = Array.from(input.files ?? []).map((f) => f.name);
			resolve(files);
		};
		input.click();
	});
}

export function saveFileDialog(
	options?: SaveDialogOptions,
): Promise<string | null> {
	if (isDesktop()) {
		return bridge.invoke<string | null>(command("Dialog", "Save"), options);
	}
	return Promise.resolve(null);
}

export function showMessageBox(options: MessageBoxOptions): Promise<number> {
	if (isDesktop()) {
		return bridge.invoke<number>(command("Dialog", "Message"), options);
	}
	const ok = window.confirm(
		`${options.title ? `${options.title}\n` : ""}${options.message}`,
	);
	return Promise.resolve(ok ? 0 : 1);
}
