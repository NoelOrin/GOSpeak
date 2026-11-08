import {
	Button,
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	Slider,
	toast,
} from "@gospeak/components";
import { Loader2, RotateCcw, RotateCw, ZoomIn, ZoomOut } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import Cropper, { type Area, type Point } from "react-easy-crop";
import {
	AVATAR_OUTPUT_SIZE,
	canvasToPngBlob,
	renderRoundAvatar,
} from "./cropImage";

interface AvatarCropperModalProps {
	/** 待裁切的图片文件；为 null 时弹窗不渲染 */
	file: File | null;
	/** 是否打开 */
	open: boolean;
	/** 取消裁切 */
	onCancel: () => void;
	/** 确认裁切，返回圆形 PNG Blob 与建议文件名 */
	onConfirm: (blob: Blob, fileName: string) => void;
}

const MIN_ZOOM = 1;
const MAX_ZOOM = 4;

/**
 * AvatarCropperModal — 基于 react-easy-crop 的头像裁切弹窗。
 *
 * 交互由库托管：鼠标/触控拖拽、双指缩放与旋转、滚轮缩放、方向键微调，
 * 并限制图片位置，保证圆形裁剪框内始终铺满、不会露出空白。
 * 确认时按裁剪区域导出 512×512 的圆形 PNG。
 */
export default function AvatarCropperModal({
	file,
	open,
	onCancel,
	onConfirm,
}: AvatarCropperModalProps) {
	const [imageSrc, setImageSrc] = useState<string | null>(null);
	const [loadState, setLoadState] = useState<"loading" | "ready" | "error">(
		"loading",
	);
	const [crop, setCrop] = useState<Point>({ x: 0, y: 0 });
	const [zoom, setZoom] = useState(MIN_ZOOM);
	const [rotation, setRotation] = useState(0);
	const [croppedArea, setCroppedArea] = useState<Area | null>(null);
	const [exporting, setExporting] = useState(false);

	// 每次打开（或换文件）都重新创建 Object URL，关闭时释放。
	useEffect(() => {
		if (!open || !file) return;
		const url = URL.createObjectURL(file);
		setImageSrc(url);
		setLoadState("loading");
		setCrop({ x: 0, y: 0 });
		setZoom(MIN_ZOOM);
		setRotation(0);
		setCroppedArea(null);
		return () => {
			URL.revokeObjectURL(url);
		};
	}, [open, file]);

	const handleCropComplete = useCallback((_area: Area, pixels: Area) => {
		setCroppedArea(pixels);
	}, []);

	const handleMediaLoaded = useCallback(() => setLoadState("ready"), []);

	const handleImageError = useCallback(() => setLoadState("error"), []);

	const handleReset = () => {
		setCrop({ x: 0, y: 0 });
		setZoom(MIN_ZOOM);
		setRotation(0);
	};

	const handleConfirm = async () => {
		if (!imageSrc || !croppedArea || exporting) return;
		setExporting(true);
		try {
			const canvas = await renderRoundAvatar(imageSrc, croppedArea, rotation);
			const blob = await canvasToPngBlob(canvas);
			const base = file?.name.replace(/\.[^.]+$/, "") || "avatar";
			onConfirm(blob, `${base}.png`);
		} catch (e) {
			toast.error(e instanceof Error ? e.message : "头像裁切失败");
		} finally {
			setExporting(false);
		}
	};

	const busy = loadState !== "ready" || exporting;

	return (
		<Dialog
			open={open && !!file}
			onOpenChange={(next) => {
				if (!next) onCancel();
			}}
		>
			<DialogContent className="max-w-[420px]">
				<DialogHeader>
					<DialogTitle>裁剪头像</DialogTitle>
					<DialogDescription>
						拖动图片调整位置，用滚轮或滑块缩放，也可旋转后再裁切。
					</DialogDescription>
				</DialogHeader>

				<div className="relative h-[320px] w-full overflow-hidden rounded-sm bg-black">
					{imageSrc && loadState !== "error" && (
						<Cropper
							image={imageSrc}
							crop={crop}
							zoom={zoom}
							rotation={rotation}
							aspect={1}
							cropShape="round"
							showGrid={false}
							objectFit="cover"
							minZoom={MIN_ZOOM}
							maxZoom={MAX_ZOOM}
							zoomSpeed={1}
							zoomWithScroll
							restrictPosition
							roundCropAreaPixels
							keyboardStep={1}
							onCropChange={setCrop}
							onZoomChange={setZoom}
							onCropComplete={handleCropComplete}
							onMediaLoaded={handleMediaLoaded}
							style={{
								cropAreaStyle: {
									border: "2px solid rgba(255, 255, 255, 0.9)",
								},
							}}
							classes={{}}
							mediaProps={{ alt: "", onError: handleImageError }}
							cropperProps={{}}
						/>
					)}

					{loadState !== "ready" && (
						<div className="absolute inset-0 flex flex-col items-center justify-center gap-2 text-sm text-white/80">
							{loadState === "loading" ? (
								<>
									<Loader2 className="size-5 animate-spin" />
									<span>图片加载中…</span>
								</>
							) : (
								<>
									<span>图片无法加载，请换一张重试</span>
									<Button
										type="button"
										variant="outline"
										size="sm"
										onClick={onCancel}
									>
										关闭
									</Button>
								</>
							)}
						</div>
					)}
				</div>

				<div className="flex flex-col gap-3">
					<div className="flex items-center gap-3">
						<ZoomOut className="text-muted-foreground size-4 shrink-0" />
						<Slider
							value={[zoom]}
							min={MIN_ZOOM}
							max={MAX_ZOOM}
							step={0.01}
							onValueChange={(value) => setZoom(value[0] ?? MIN_ZOOM)}
							disabled={busy}
							aria-label="缩放"
						/>
						<ZoomIn className="text-muted-foreground size-4 shrink-0" />
					</div>

					<div className="flex items-center justify-between gap-2">
						<div className="flex items-center gap-2">
							<Button
								type="button"
								variant="outline"
								size="sm"
								onClick={() => setRotation((r) => (r + 270) % 360)}
								disabled={busy}
								title="向左旋转 90°"
								aria-label="向左旋转 90°"
							>
								<RotateCcw className="size-4" />
							</Button>
							<Button
								type="button"
								variant="outline"
								size="sm"
								onClick={() => setRotation((r) => (r + 90) % 360)}
								disabled={busy}
								title="向右旋转 90°"
								aria-label="向右旋转 90°"
							>
								<RotateCw className="size-4" />
							</Button>
							<Button
								type="button"
								variant="ghost"
								size="sm"
								onClick={handleReset}
								disabled={busy}
							>
								重置
							</Button>
						</div>
						<span className="text-muted-foreground text-xs whitespace-nowrap">
							输出 {AVATAR_OUTPUT_SIZE}×{AVATAR_OUTPUT_SIZE} PNG
						</span>
					</div>
				</div>

				<div className="flex justify-end gap-2">
					<Button
						type="button"
						variant="outline"
						onClick={onCancel}
						disabled={exporting}
					>
						取消
					</Button>
					<Button
						type="button"
						onClick={() => void handleConfirm()}
						disabled={!croppedArea || busy}
					>
						{exporting && <Loader2 className="size-4 animate-spin" />}
						确认
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
