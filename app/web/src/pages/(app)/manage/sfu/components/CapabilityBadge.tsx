export interface CapabilityBadgeProps {
	label: string;
	active: boolean;
}

export default function CapabilityBadge(props: CapabilityBadgeProps) {
	return (
		<span
			class="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium"
			classList={{
				"border-base-300 bg-base-100 text-base-content/75": props.active,
				"border-base-300 bg-base-100 text-base-content/40": !props.active,
			}}
		>
			<span
				class="inline-block size-1.5 rounded-full"
				classList={{
					"bg-base-content/55": props.active,
					"bg-base-content/25": !props.active,
				}}
			/>
			{props.label}
		</span>
	);
}
