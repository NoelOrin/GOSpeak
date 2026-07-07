import SearchModal from "@/components/modal/searchModal";
import SvgIcon from "./svgIcon";

const FuncButton = ({
	label,
	onClick,
}: {
	label?: string;
	onClick?: () => void;
}) => {
	let searchModalRef!: HTMLDialogElement;

	const openModal = () => {
		searchModalRef.showModal();
	};

	const closeModal = () => {
		searchModalRef.close();
	};

	return (
		<>
			<div class="absolute fab">
				{/* a focusable div with tabIndex is necessary to work on all browsers. role="button" is necessary for accessibility */}
				<div
					tabIndex={0}
					role="button"
					class="dark:text-white btn btn-lg btn-circle btn-accent"
				>
					<svg
						width="24"
						height="24"
						viewBox="0 0 48 48"
						fill="none"
						xmlns="http://www.w3.org/2000/svg"
					>
						<path
							d="M24.0605 10L24.0239 38"
							stroke="currentColor"
							stroke-width="4"
							stroke-linecap="butt"
							stroke-linejoin="miter"
						/>
						<path
							d="M10 24L38 24"
							stroke="currentColor"
							stroke-width="4"
							stroke-linecap="butt"
							stroke-linejoin="miter"
						/>
					</svg>
				</div>

				{/* close button should not be focusable so it can close the FAB when clicked. It's just a visual placeholder */}
				<div class="fab-close">
					<span class="dark:text-white btn btn-circle btn-lg btn-error">✕</span>
				</div>

				{/* buttons that show up when FAB is open */}
				<div>
					连接服务器
					<button
						class="dark:text-white btn btn-lg btn-circle btn-accent"
						onClick={openModal}
					>
						<SvgIcon name="connect" />
					</button>
				</div>
			</div>
			<SearchModal ref={searchModalRef} onClose={closeModal} />
		</>
	);
};
export default FuncButton;
