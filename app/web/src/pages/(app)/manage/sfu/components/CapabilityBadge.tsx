export interface CapabilityBadgeProps {
	label: string;
	active: boolean;
}

export default function CapabilityBadge(props: CapabilityBadgeProps) {
	return (
		<div
			class="rounded-box flex items-center justify-center gap-2 border px-3 py-2 text-xs"
			classList={{
				"border-base-300 bg-base-200/70 text-base-content/75": props.active,
				"border-base-300 bg-base-100 text-base-content/45": !props.active,
			}}
		>
			<span
				class="inline-block size-2 rounded-full"
				classList={{
					"bg-base-content/55": props.active,
					"bg-base-content/30": !props.active,
				}}
			/>
			<span>{props.label}</span>
		</div>
	);
}
