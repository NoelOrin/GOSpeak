import type { Component } from "solid-js";

interface GuildIconProps {
	name: string;
	iconUrl?: string;
	active?: boolean;
	onClick?: () => void;
	class?: string;
}

const GuildIcon: Component<GuildIconProps> = (props) => {
	const initials = () => props.name.slice(0, 2).toUpperCase();
	return (
		<button
			onClick={props.onClick}
			class={props.class || ""}
			classList={{
				"w-12 h-12 rounded-2xl flex items-center justify-center text-white font-bold text-lg transition-all cursor-pointer hover:rounded-xl": true,
				"bg-blue-500": !props.active,
				"bg-blue-600 rounded-xl": !!props.active,
			}}
			title={props.name}
		>
			{props.iconUrl ? (
				<img
					src={props.iconUrl}
					alt={props.name}
					class="w-12 h-12 rounded-2xl object-cover"
				/>
			) : (
				<span>{initials()}</span>
			)}
		</button>
	);
};

export default GuildIcon;
