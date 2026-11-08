import type { Area } from "react-easy-crop";

/** 头像导出边长（正方形，圆形裁剪后输出） */
export const AVATAR_OUTPUT_SIZE = 512;

/** 旋转画布最大边长：防止超大原图直接进 canvas 导致内存暴涨 */
const MAX_STAGE_SIZE = 4096;

/** 加载图片。浏览器会自动应用 EXIF 方向，与 Cropper 内部的 <img> 保持一致。 */
export function loadImage(src: string): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const img = new Image();
		img.onload = () => resolve(img);
		img.onerror = () => reject(new Error("图片加载失败，请换一张图片重试"));
		img.src = src;
	});
}

/** 计算图片旋转后的外接矩形尺寸 */
export function rotateSize(
	width: number,
	height: number,
	rotation: number,
): { width: number; height: number } {
	const rad = (rotation * Math.PI) / 180;
	const cos = Math.abs(Math.cos(rad));
	const sin = Math.abs(Math.sin(rad));
	return {
		width: width * cos + height * sin,
		height: width * sin + height * cos,
	};
}

/**
 * 生成圆形头像画布。
 *
 * `crop` 是 react-easy-crop 的 `croppedAreaPixels`：它是相对于「旋转后的图片」
 * 的像素坐标，所以这里先按 rotation 把原图画到一张旋转外接画布上，再取裁剪区域。
 * 输出为带透明背景的圆形 PNG 画布（圆形外区域透明，避免方形黑边）。
 */
export async function renderRoundAvatar(
	src: string,
	crop: Area,
	rotation = 0,
	outputSize = AVATAR_OUTPUT_SIZE,
): Promise<HTMLCanvasElement> {
	const img = await loadImage(src);
	const rad = (rotation * Math.PI) / 180;
	const rotated = rotateSize(img.naturalWidth, img.naturalHeight, rotation);
	const scale = Math.min(
		1,
		MAX_STAGE_SIZE / Math.max(rotated.width, rotated.height),
	);

	// 1) 旋转后的完整图片（等比降采样，避免大图 OOM）
	const stage = document.createElement("canvas");
	stage.width = Math.max(1, Math.ceil(rotated.width * scale));
	stage.height = Math.max(1, Math.ceil(rotated.height * scale));
	const stageCtx = stage.getContext("2d");
	if (!stageCtx) throw new Error("无法创建画布上下文");
	stageCtx.imageSmoothingEnabled = true;
	stageCtx.imageSmoothingQuality = "high";
	stageCtx.translate(stage.width / 2, stage.height / 2);
	stageCtx.rotate(rad);
	stageCtx.scale(scale, scale);
	stageCtx.drawImage(img, -img.naturalWidth / 2, -img.naturalHeight / 2);

	// 2) 取裁剪区域 → 缩放到输出尺寸 → 圆形遮罩
	const output = document.createElement("canvas");
	output.width = outputSize;
	output.height = outputSize;
	const ctx = output.getContext("2d");
	if (!ctx) throw new Error("无法创建画布上下文");
	ctx.imageSmoothingEnabled = true;
	ctx.imageSmoothingQuality = "high";
	ctx.save();
	ctx.beginPath();
	ctx.arc(outputSize / 2, outputSize / 2, outputSize / 2, 0, Math.PI * 2);
	ctx.closePath();
	ctx.clip();
	ctx.drawImage(
		stage,
		crop.x * scale,
		crop.y * scale,
		crop.width * scale,
		crop.height * scale,
		0,
		0,
		outputSize,
		outputSize,
	);
	ctx.restore();

	return output;
}

/** 画布转 PNG Blob */
export function canvasToPngBlob(canvas: HTMLCanvasElement): Promise<Blob> {
	return new Promise((resolve, reject) => {
		canvas.toBlob((blob) => {
			if (blob) resolve(blob);
			else reject(new Error("头像导出失败"));
		}, "image/png");
	});
}
