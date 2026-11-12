export function defaultCanvasRender(
	ctx: CanvasRenderingContext2D,
	canvas: HTMLCanvasElement,
): void {
	const w = canvas.width;
	const h = canvas.height;

	ctx.fillStyle = "#1a1a2e";
	ctx.fillRect(0, 0, w, h);

	ctx.fillStyle = "#4ade80";
	ctx.beginPath();
	ctx.arc(w / 2, h / 2 - 10, 20, 0, Math.PI * 2);
	ctx.fill();

	ctx.fillStyle = "#ffffff";
	ctx.font = "bold 14px sans-serif";
	ctx.textAlign = "center";
	ctx.fillText("GOSpeak", w / 2, h - 12);
}
