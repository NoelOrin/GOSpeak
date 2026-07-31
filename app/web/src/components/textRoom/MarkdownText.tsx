function escapeHtml(value: string): string {
	return value
		.replaceAll("&", "&amp;")
		.replaceAll("<", "&lt;")
		.replaceAll(">", "&gt;")
		.replaceAll('"', "&quot;")
		.replaceAll("'", "&#039;");
}

export function renderMarkdown(value: string): string {
	const escaped = escapeHtml(value);
	const blocks: string[] = [];
	const withBlocks = escaped.replace(
		/```([\s\S]*?)```/g,
		(_match, code: string) => {
			blocks.push(
				`<pre class="bg-base-200 rounded-lg p-2 overflow-x-auto text-xs"><code>${code}</code></pre>`,
			);
			return `@@GOSPEAK_BLOCK_${blocks.length - 1}@@`;
		},
	);
	return withBlocks
		.replace(/(?:^|\n)#### ([^\n]+)/g, "\n<h4>$1</h4>")
		.replace(/(?:^|\n)### ([^\n]+)/g, "\n<h3>$1</h3>")
		.replace(/(?:^|\n)## ([^\n]+)/g, "\n<h2>$1</h2>")
		.replace(/(?:^|\n)# ([^\n]+)/g, "\n<h1>$1</h1>")
		.replace(/\n/g, "<br />")
		.replace(/@@GOSPEAK_BLOCK_(\d+)@@/g, (_match, index: string) => {
			return blocks[Number(index)] || "";
		});
}

export default function MarkdownText(props: { text: string; class?: string }) {
	const html = () => renderMarkdown(props.text);
	return (
		<div
			class={`text-sm break-words whitespace-pre-wrap [&_a]:text-primary [&_a]:underline ${props.class || ""}`}
			innerHTML={html()}
		/>
	);
}
