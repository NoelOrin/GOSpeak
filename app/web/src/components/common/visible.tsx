import { mergeProps } from "solid-js";
import type { JSX } from "solid-js/jsx-runtime";

interface VisibleProps {
	when: boolean;
	children: JSX.Element;
	as?: string;
}

const Visible = (props: VisibleProps) => {
	const merged = mergeProps({ as: "div" }, props);
	const Tag = merged.as as keyof JSX.IntrinsicElements;

	return (
		<Tag style={{ display: merged.when ? "" : "none" }}>{merged.children}</Tag>
	);
};

export default Visible;
