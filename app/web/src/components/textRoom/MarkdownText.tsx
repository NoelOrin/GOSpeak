import DOMPurify from "dompurify";
import { marked } from "marked";

marked.setOptions({
	gfm: true,
	breaks: true,
});

export function renderMarkdown(value: string): string {
	const raw = marked.parse(value || "", { async: false }) as string;
	return DOMPurify.sanitize(raw);
}

export default function MarkdownText(props: { text: string; class?: string }) {
	const html = () => renderMarkdown(props.text);
	return (
		<div
			class={`text-sm break-words [&_a]:text-primary [&_a]:underline [&_blockquote]:border-l-2 [&_blockquote]:border-base-300 [&_blockquote]:pl-3 [&_blockquote]:text-base-content/70 [&_code]:rounded [&_code]:bg-base-200 [&_code]:px-1 [&_code]:py-0.5 [&_h1]:mb-2 [&_h1]:text-lg [&_h1]:font-semibold [&_h2]:mb-2 [&_h2]:text-base [&_h2]:font-semibold [&_h3]:mb-2 [&_h3]:text-sm [&_h3]:font-semibold [&_img]:max-w-full [&_img]:rounded-lg [&_img]:border [&_img]:border-base-300 [&_li]:ml-4 [&_ol]:list-decimal [&_pre]:my-2 [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:bg-base-200 [&_pre]:p-2 [&_pre]:text-xs [&_pre]:leading-5 [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_ul]:list-disc ${props.class || ""}`}
			innerHTML={html()}
		/>
	);
}
