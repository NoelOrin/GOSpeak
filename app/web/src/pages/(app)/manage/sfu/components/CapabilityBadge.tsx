export interface CapabilityBadgeProps {
	label: string;
	active: boolean;
}

export default function CapabilityBadge({
	label,
	active,
}: CapabilityBadgeProps) {
	return (
		<span
			className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium ${
				active
					? "border-border bg-card text-foreground"
					: "border-border bg-card text-faint-foreground"
			}`}
		>
			<span
				className={`inline-block size-1.5 rounded-full ${active ? "bg-foreground/55" : "bg-foreground/25"}`}
			/>
			{label}
		</span>
	);
}
