import type { JSX } from "solid-js";

interface StatsCardProps {
	title: string;
	value: string | number;
	description: string;
	icon: JSX.Element;
	accentClass?: string;
}

const StatsCard = (props: StatsCardProps) => {
	return (
		<div class="min-w-0 rounded-lg border border-base-300 bg-base-100 p-5 shadow-sm">
			<div class="flex min-w-0 items-start justify-between gap-4">
				<div class="min-w-0 space-y-2">
					<div class="min-w-0 break-words text-sm text-base-content/60">
						{props.title}
					</div>
					<div class="min-w-0 break-words text-3xl font-semibold leading-none">
						{props.value}
					</div>
					<div class="min-w-0 break-words text-sm text-base-content/70">
						{props.description}
					</div>
				</div>
				<div
					class={`flex h-11 w-11 items-center justify-center rounded-lg bg-base-200 ${props.accentClass || "text-primary"}`}
				>
					{props.icon}
				</div>
			</div>
		</div>
	);
};

export default StatsCard;
