import type { JSX } from "solid-js";

export const Toggle = (props: {
	label: string;
	desc?: string;
	checked: boolean;
	disabled?: boolean;
	onChange: (v: boolean) => void;
}) => (
	<div class="flex items-center justify-between gap-4 py-2">
		<div class="min-w-0">
			<div class="text-sm font-medium">{props.label}</div>
			{props.desc ? (
				<div class="text-xs text-base-content/50">{props.desc}</div>
			) : null}
		</div>
		<input
			type="checkbox"
			class="toggle toggle-sm shrink-0"
			checked={props.checked}
			disabled={props.disabled}
			onChange={(e) => props.onChange(e.target.checked)}
		/>
	</div>
);

export const Section = (props: {
	title: string;
	children: JSX.Element;
	action?: JSX.Element;
}) => (
	<section class="flex flex-col gap-2">
		<div class="flex items-center justify-between gap-2">
			<div class="divider my-0 flex-1 text-xs text-base-content/40">
				{props.title}
			</div>
			{props.action}
		</div>
		{props.children}
	</section>
);

export const Field = (props: {
	label: string;
	hint?: string;
	children: JSX.Element;
}) => (
	<fieldset class="fieldset">
		<legend class="fieldset-legend text-[14px]">{props.label}</legend>
		{props.children}
		{props.hint ? (
			<p class="mt-1 text-xs text-base-content/45">{props.hint}</p>
		) : null}
	</fieldset>
);

export const Page = (props: {
	title: string;
	desc?: string;
	children: JSX.Element;
}) => (
	<div class="flex h-full flex-col gap-4 p-5 sm:p-6">
		<div>
			<h3 class="text-lg font-bold">{props.title}</h3>
			{props.desc ? (
				<p class="mt-1 text-sm text-base-content/55">{props.desc}</p>
			) : null}
		</div>
		{props.children}
	</div>
);
