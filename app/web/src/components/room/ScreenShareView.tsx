import type { RemoteVideoTrackLike } from "@gospeak/sfu-client/types";
import { Maximize2, Minimize2 } from "lucide-react";
import {
	forwardRef,
	useEffect,
	useImperativeHandle,
	useRef,
	useState,
} from "react";

/**
 * 把 provider 的视频 track 附到一个 <video> 上。
 * 屏幕共享本身不带音频（音频走独立的 audio track），所以默认静音播放。
 *
 * 外层容器按视频实际画面比例收缩（object-contain 不留黑边），全屏按钮因此
 * 始终贴在画面右下角；forwardRef 暴露 <video> 本身，供外层对它调用
 * requestFullscreen()。
 */
const ScreenShareView = forwardRef<
	HTMLVideoElement,
	{
		track: RemoteVideoTrackLike;
		className?: string;
		onClick?: () => void;
		fullscreen?: boolean;
		onToggleFullscreen?: () => void;
	}
>(
	(
		{ track, className, onClick, fullscreen = false, onToggleFullscreen },
		ref,
	) => {
		const videoRef = useRef<HTMLVideoElement>(null);
		const [aspectRatio, setAspectRatio] = useState<string | null>(null);
		useImperativeHandle(ref, () => videoRef.current as HTMLVideoElement, []);

		useEffect(() => {
			const el = videoRef.current;
			if (!el) return;
			setAspectRatio(null);
			track.attach(el);

			const play = () => {
				void el.play().catch(() => {});
			};
			const syncRatio = () => {
				if (!el.videoWidth || !el.videoHeight) return;
				setAspectRatio(`${el.videoWidth} / ${el.videoHeight}`);
			};

			el.addEventListener("loadedmetadata", play);
			el.addEventListener("loadedmetadata", syncRatio);
			el.addEventListener("resize", syncRatio);
			play();
			syncRatio();
			return () => {
				el.removeEventListener("loadedmetadata", play);
				el.removeEventListener("loadedmetadata", syncRatio);
				el.removeEventListener("resize", syncRatio);
				track.detach(el);
			};
		}, [track]);

		return (
			<div
				className={`relative flex max-h-full max-w-full min-h-0 min-w-0 overflow-hidden ${className ?? ""}`}
				style={{ aspectRatio: aspectRatio ?? "16 / 9" }}
			>
				<video
					ref={videoRef}
					autoPlay
					playsInline
					muted
					className="block h-full w-full object-contain"
					onClick={onClick}
				/>
				{onToggleFullscreen && (
					<button
						type="button"
						className="absolute right-3 bottom-3 flex size-8 cursor-pointer items-center justify-center rounded-sm bg-black/55 text-white transition-colors hover:bg-black/75"
						title={fullscreen ? "退出全屏" : "全屏"}
						aria-label={fullscreen ? "退出全屏" : "全屏"}
						onClick={(event) => {
							event.stopPropagation();
							onToggleFullscreen();
						}}
					>
						{fullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
					</button>
				)}
			</div>
		);
	},
);

ScreenShareView.displayName = "ScreenShareView";

export default ScreenShareView;
