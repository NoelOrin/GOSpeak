import { type JSX, Show } from "solid-js";

export interface FormFieldProps {
	label: string;
	error?: string;
	children: JSX.Element;
}

/** 管理页通用表单字段：label + children + error。 */
export default function FormField(props: FormFieldProps) {
	return (
		<fieldset class="fieldset">
			<legend class="fieldset-legend text-[14px]">{props.label}</legend>
			{props.children}
			<Show when={props.error}>
				<p class="mt-1 text-xs text-error">{props.error}</p>
			</Show>
		</fieldset>
	);
}
