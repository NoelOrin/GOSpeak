import clsx from "clsx";
import { Show } from "solid-js";

interface AvatarProps {
	src?: string | null;
	name?: string | null;
	alt?: string;
	class?: string;
	textClass?: string;
}

const Avatar = (props: AvatarProps) => {
	const initial = () => {
		const n = props.name?.trim();
		return (n ? n.charAt(0) : "?").toUpperCase();
	};
	const sizeClass = () => props.class ?? "size-10";

	return (
		<Show
			when={props.src}
			fallback={
				<div
					class={clsx(
						"flex justify-center items-center rounded-full bg-linear-to-br from-primary to-secondary text-primary-content font-bold shrink-0",
						sizeClass(),
						props.textClass ?? "text-base",
					)}
				>
					{initial()}
				</div>
			}
		>
			<div class="avatar shrink-0">
				<div class={clsx("rounded-full", sizeClass())}>
					<img
						src={props.src ?? undefined}
						alt={props.alt ?? props.name ?? ""}
					/>
				</div>
			</div>
		</Show>
	);
};

export default Avatar;
