import type { Component } from "solid-js";
import OptionSquare from "@/components/common/optionSquare";

interface DomainIconProps {
	name: string;
	iconUrl?: string;
	active?: boolean;
	onClick?: () => void;
	class?: string;
}

const DomainIcon: Component<DomainIconProps> = (props) => {
	const initials = () => props.name.slice(0, 2).toUpperCase();
	return (
		<OptionSquare
			label={props.name}
			onClick={props.onClick}
			class={props.class}
		>
			{props.iconUrl ? (
				<img
					src={props.iconUrl}
					alt={props.name}
					class="w-12 h-12 rounded-2xl object-cover"
				/>
			) : (
				<span class="text-lg font-bold">{initials()}</span>
			)}
		</OptionSquare>
	);
};

export default DomainIcon;
