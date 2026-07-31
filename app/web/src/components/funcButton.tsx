import { createSignal } from "solid-js";
import SearchModal from "@/components/modal/searchModal";
import SvgIcon from "./svgIcon";

const FuncButton = () => {
	let searchModalRef!: HTMLDialogElement;
	let fabMainRef!: HTMLButtonElement;
	const [fabOpen, setFabOpen] = createSignal(false);

	const openModal = () => {
		searchModalRef.showModal();
	};

	const closeModal = () => {
		searchModalRef.close();
	};

	const closeFab = (event: { currentTarget: HTMLButtonElement }) => {
		event.currentTarget.blur();
		setFabOpen(false);
	};

	return (
		<>
			<div
				class="absolute fab"
				onFocusIn={() => setFabOpen(true)}
				onFocusOut={(event) => {
					if (
						!event.currentTarget.contains(event.relatedTarget as Node | null)
					) {
						setFabOpen(false);
					}
				}}
			>
				<button
					type="button"
					ref={fabMainRef}
					tabIndex={0}
					aria-label="打开操作菜单"
					aria-haspopup="menu"
					aria-expanded={fabOpen()}
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
				</button>

				<button
					type="button"
					class="fab-close"
					aria-label="关闭操作菜单"
					onClick={closeFab}
				>
					<span class="dark:text-white btn btn-circle btn-lg btn-error">✕</span>
				</button>

				<div>
					连接服务器
					<button
						type="button"
						class="dark:text-white btn btn-lg btn-circle btn-accent"
						aria-label="连接服务器"
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
