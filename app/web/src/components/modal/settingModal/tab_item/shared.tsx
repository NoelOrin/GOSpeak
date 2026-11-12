import { Switch } from "@gospeak/components";
import type { ReactNode } from "react";

export const Toggle = ({
	label,
	desc,
	checked,
	disabled,
	onChange,
}: {
	label: string;
	desc?: string;
	checked: boolean;
	disabled?: boolean;
	onChange: (v: boolean) => void;
}) => (
	<div className="flex items-center justify-between gap-4 py-2">
		<div className="min-w-0">
			<div className="text-sm font-medium">{label}</div>
			{desc ? (
				<div className="text-faint-foreground text-xs">{desc}</div>
			) : null}
		</div>
		<Switch
			checked={checked}
			disabled={disabled}
			onCheckedChange={onChange}
			aria-label={label}
		/>
	</div>
);

export const Section = ({
	title,
	desc,
	children,
	action,
}: {
	title: string;
	desc?: string;
	children: ReactNode;
	action?: ReactNode;
}) => (
	<section className="flex flex-col gap-2">
		<div className="flex items-center justify-between gap-2">
			<div className="text-faint-foreground flex flex-1 items-center gap-2 text-xs">
				<span className="bg-border h-px flex-1" />
				{title}
				<span className="bg-border h-px flex-1" />
			</div>
			{action}
		</div>
		{desc ? <p className="text-faint-foreground text-xs">{desc}</p> : null}
		{children}
	</section>
);

export const Field = ({
	label,
	desc,
	hint,
	children,
}: {
	label: string;
	desc?: string;
	hint?: string;
	children: ReactNode;
}) => (
	<label className="block">
		<span className="mb-1 block text-sm">{label}</span>
		{desc ? (
			<span className="text-faint-foreground mb-1 block text-xs">{desc}</span>
		) : null}
		{children}
		{hint ? <p className="text-faint-foreground mt-1 text-xs">{hint}</p> : null}
	</label>
);

export const Page = ({
	title,
	desc,
	children,
}: {
	title: string;
	desc?: string;
	children: ReactNode;
}) => (
	<div className="flex h-full flex-col gap-4 p-5 sm:p-6">
		<div>
			<h3 className="text-lg font-bold">{title}</h3>
			{desc ? (
				<p className="text-muted-foreground mt-1 text-sm">{desc}</p>
			) : null}
		</div>
		{children}
	</div>
);
