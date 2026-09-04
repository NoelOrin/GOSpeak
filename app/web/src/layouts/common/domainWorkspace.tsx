import { createSignal, Show } from "solid-js";
import MemberSidebar from "@/components/room/components/memberSidebar";
import RoomDetail from "@/components/room/roomDetail";
import TextRoomPanel from "@/components/textRoom/TextRoomPanel";

interface DomainWorkspaceProps {
	isDomain: () => boolean;
	mobileHideMembers?: boolean;
}

const DomainWorkspace = ({
	isDomain,
	mobileHideMembers = false,
}: DomainWorkspaceProps) => {
	let dragRef!: HTMLDivElement;

	const [verticalSplit, setVerticalSplit] = createSignal(
		localStorage.getItem("vertSplit") || "50%",
	);

	const onDragMouseDown = (e: MouseEvent) => {
		e.preventDefault();
		const startY = e.clientY;
		const startRatio = parseFloat(verticalSplit()) / 100;
		const containerHeight = dragRef.parentElement?.offsetHeight;
		if (!containerHeight) return;

		const onMouseMove = (e: MouseEvent) => {
			const delta = e.clientY - startY;
			const newRatio = (startRatio + delta / containerHeight) * 100;
			const clamped = Math.max(10, Math.min(90, newRatio));
			setVerticalSplit(`${clamped}%`);
			localStorage.setItem("vertSplit", `${clamped}%`);
		};

		const onMouseUp = () => {
			document.removeEventListener("mousemove", onMouseMove);
			document.removeEventListener("mouseup", onMouseUp);
		};

		document.addEventListener("mousemove", onMouseMove);
		document.addEventListener("mouseup", onMouseUp);
	};

	return (
		<div
			class="flex-1 border-color border-t w-full h-full bg-base-200"
			style={{ display: isDomain() ? undefined : "none" }}
		>
			<div class="flex w-full h-full">
				<div class="flex flex-col h-full flex-1">
					<div class="overflow-hidden" style={{ height: verticalSplit() }}>
						<RoomDetail />
					</div>
					{/** biome-ignore lint/a11y/noStaticElementInteractions: drag handle is pointer-only */}
					<div
						class="h-1.5 bg-base-300 hover:bg-primary cursor-row-resize rounded-full mx-2 shrink-0"
						ref={dragRef}
						onMouseDown={onDragMouseDown}
						title="拖动调整语音与文字高度"
					/>
					<div class="flex-1 overflow-hidden">
						<TextRoomPanel />
					</div>
				</div>
				<Show when={!mobileHideMembers}>
					<MemberSidebar />
				</Show>
			</div>
		</div>
	);
};

export default DomainWorkspace;
