import OptionSquare from "@/components/common/optionSquare";

interface DomainIconProps {
	name: string;
	iconUrl?: string;
	active?: boolean;
	onClick?: () => void;
	requiresDoubleClick?: boolean;
	className?: string;
}

export default function DomainIcon({
	name,
	iconUrl,
	active,
	onClick,
	requiresDoubleClick,
	className,
}: DomainIconProps) {
	// 与 Avatar 保持一致：域头像只取名称的首个字符（按 Unicode 码点，兼容 emoji）
	const initial = (Array.from(name.trim())[0] ?? "?").toUpperCase();
	return (
		<OptionSquare
			label={name}
			onClick={onClick}
			onDoubleClick={onClick}
			active={active}
			requiresDoubleClick={requiresDoubleClick}
			actionHint="双击进入"
			className={className}
		>
			{iconUrl ? (
				<img
					src={iconUrl}
					alt={name}
					className="size-12 rounded-xl object-cover"
				/>
			) : (
				<span className="text-xl font-bold">{initial}</span>
			)}
		</OptionSquare>
	);
}
