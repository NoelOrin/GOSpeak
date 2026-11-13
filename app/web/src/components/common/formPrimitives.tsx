import { cn, Switch } from "@gospeak/components";
import type { ReactNode } from "react";
import Divider from "@/components/common/divider";

export { Divider };

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
	<div className="flex items-center justify-between gap-4 py-1.5">
		<div className="min-w-0">
			<div className="text-foreground text-sm font-medium">{label}</div>
			{desc ? (
				<div className="text-faint-foreground mt-0.5 text-xs leading-relaxed">
					{desc}
				</div>
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
	<section className="flex flex-col gap-3">
		<div className="flex items-start justify-between gap-3">
			<div className="min-w-0 flex-1">
				<h4 className="text-foreground text-sm font-semibold">{title}</h4>
				{desc ? (
					<p className="text-faint-foreground mt-0.5 text-xs leading-relaxed">
						{desc}
					</p>
				) : null}
			</div>
			{action ? <div className="shrink-0 pt-0.5">{action}</div> : null}
		</div>
		<div className="flex flex-col gap-3">{children}</div>
	</section>
);

export const Row = ({
	label,
	desc,
	children,
}: {
	label: string;
	desc?: string;
	children: ReactNode;
}) => (
	<div className="flex items-start justify-between gap-4 py-1.5">
		<div className="min-w-0 flex-1">
			<div className="text-foreground text-sm font-medium">{label}</div>
			{desc ? (
				<div className="text-faint-foreground mt-0.5 text-xs leading-relaxed">
					{desc}
				</div>
			) : null}
		</div>
		<div className="shrink-0">{children}</div>
	</div>
);

export const ReadonlyRow = ({
	label,
	value,
	desc,
	badge,
}: {
	label: string;
	value: ReactNode;
	desc?: string;
	badge?: ReactNode;
}) => (
	<div className="flex items-start justify-between gap-4 py-1.5">
		<div className="min-w-0 flex-1">
			<div className="text-foreground text-sm font-medium">{label}</div>
			{desc ? (
				<div className="text-faint-foreground mt-0.5 text-xs leading-relaxed">
					{desc}
				</div>
			) : null}
		</div>
		<div className="flex shrink-0 items-center gap-2">
			<span className="text-muted-foreground max-w-[16rem] truncate text-sm">
				{value}
			</span>
			{badge}
		</div>
	</div>
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
	<label className="flex flex-col gap-1.5">
		<span className="text-foreground text-sm font-medium">{label}</span>
		{desc ? (
			<span className="text-faint-foreground text-xs leading-relaxed">
				{desc}
			</span>
		) : null}
		{children}
		{hint ? (
			<p className="text-faint-foreground text-xs leading-relaxed">{hint}</p>
		) : null}
	</label>
);

export const Page = ({
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
	<div className="flex h-full flex-col">
		<div className="border-border/60 bg-card/95 sticky top-0 z-10 border-b px-5 pt-5 pb-4 backdrop-blur sm:px-6">
			<div className="mx-auto flex w-full max-w-5xl items-start justify-between gap-3">
				<div className="min-w-0">
					<h3 className="text-lg font-bold tracking-tight">{title}</h3>
					{desc ? (
						<p className="text-muted-foreground mt-1 text-sm leading-relaxed">
							{desc}
						</p>
					) : null}
				</div>
				{action ? <div className="shrink-0">{action}</div> : null}
			</div>
		</div>
		<div className="min-h-0 flex-1 overflow-y-auto">
			<div className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-5 py-5 sm:px-8">
				{children}
			</div>
		</div>
	</div>
);

export const formInputClass = cn(
	"border-input bg-card text-foreground focus-visible:ring-ring placeholder:text-faint-foreground flex h-10 w-full rounded-sm border px-3 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50",
);
