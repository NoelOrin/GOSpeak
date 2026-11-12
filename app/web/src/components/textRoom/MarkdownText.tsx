import DOMPurify from "dompurify";
import { marked } from "marked";

marked.setOptions({
	gfm: true,
	breaks: true,
});

const MENTION_RE = /@([\w\u4e00-\u9fa5][\w\u4e00-\u9fa5.-]{0,23})/g;

/**
 * 给 @提及 加高亮。
 *
 * mentions 数据本来就已经随消息存下来、也随消息发出去，但渲染层把它当普通文本，
 * 用户根本看不出来有人在叫自己。这里在消毒后的 DOM 上做一次文本节点替换，
 * 跳过 code / pre / a，避免把代码里的 @ 和链接误伤。
 */
function highlightMentions(html: string, selfName?: string): string {
	if (typeof window === "undefined" || typeof DOMParser === "undefined") {
		return html;
	}
	const doc = new DOMParser().parseFromString(
		`<div>${html}</div>`,
		"text/html",
	);
	const root = doc.body.firstElementChild;
	if (!root) return html;

	const walker = doc.createTreeWalker(root, Node.TEXT_NODE);
	const targets: Text[] = [];
	while (walker.nextNode()) {
		const node = walker.currentNode as Text;
		const parent = node.parentElement;
		if (!parent) continue;
		if (parent.closest("code, pre, a")) continue;
		if (!node.nodeValue?.includes("@")) continue;
		targets.push(node);
	}

	for (const node of targets) {
		const text = node.nodeValue ?? "";
		MENTION_RE.lastIndex = 0;
		if (!MENTION_RE.test(text)) continue;
		MENTION_RE.lastIndex = 0;

		const frag = doc.createDocumentFragment();
		let last = 0;
		for (const m of text.matchAll(MENTION_RE)) {
			const start = m.index ?? 0;
			if (start > last) {
				frag.append(doc.createTextNode(text.slice(last, start)));
			}
			const name = m[1];
			const span = doc.createElement("span");
			const isSelf = !!selfName && name === selfName;
			span.className = isSelf
				? "bg-primary/20 text-primary rounded-xs px-1 font-medium"
				: "bg-muted text-foreground rounded-xs px-1 font-medium";
			span.textContent = m[0];
			frag.append(span);
			last = start + m[0].length;
		}
		if (last < text.length) {
			frag.append(doc.createTextNode(text.slice(last)));
		}
		node.replaceWith(frag);
	}

	return root.innerHTML;
}

export function renderMarkdown(value: string, selfName?: string): string {
	const raw = marked.parse(value || "", { async: false }) as string;
	const clean = DOMPurify.sanitize(raw);
	return highlightMentions(clean, selfName);
}

export default function MarkdownText({
	text,
	className,
	selfName,
}: {
	text: string;
	className?: string;
	selfName?: string;
}) {
	const html = renderMarkdown(text, selfName);
	return (
		<div
			className={`text-sm break-words [&_a]:text-primary [&_a]:underline [&_blockquote]:border-border [&_blockquote]:text-muted-foreground [&_blockquote]:border-l-2 [&_blockquote]:pl-3 [&_code]:bg-muted [&_code]:rounded-sm [&_code]:px-1 [&_code]:py-0.5 [&_h1]:mb-2 [&_h1]:text-lg [&_h1]:font-semibold [&_h2]:mb-2 [&_h2]:text-base [&_h2]:font-semibold [&_h3]:mb-2 [&_h3]:text-sm [&_h3]:font-semibold [&_img]:max-w-full [&_img]:rounded-lg [&_img]:border-border [&_img]:border [&_li]:ml-4 [&_ol]:list-decimal [&_pre]:bg-muted [&_pre]:my-2 [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:p-2 [&_pre]:text-xs [&_pre]:leading-5 [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_ul]:list-disc ${className || ""}`}
			// biome-ignore lint/security/noDangerouslySetInnerHtml: 内容经 DOMPurify 消毒
			dangerouslySetInnerHTML={{ __html: html }}
		/>
	);
}
