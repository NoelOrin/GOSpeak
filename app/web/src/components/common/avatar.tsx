import clsx from "clsx";
import { useEffect, useState } from "react";

interface AvatarProps {
	/** 头像图片地址；为空或加载失败时回退为首字母 */
	src?: string | null;
	/** 用户名/显示名，用于取首字作为 fallback */
	name?: string | null;
	alt?: string;
	/** 尺寸与额外样式（如 size-10、ring-2 等） */
	className?: string;
	/** fallback 首字母的字号 */
	textClass?: string;
}

/** 取用户名首字（Array.from 以正确处理 emoji / 代理对，中文取第一个汉字）。 */
function initialOf(name?: string | null): string {
	const trimmed = name?.trim();
	if (!trimmed) return "?";
	const first = Array.from(trimmed)[0];
	return first ? first.toUpperCase() : "?";
}

/**
 * 全局头像组件：有 src 显示图片；无 src 或图片加载失败（onError）时，
 * 统一回退为“用户名首字 + 主题渐变”的占位头像，保证全站一致。
 */
export default function Avatar({
	src,
	name,
	alt,
	className,
	textClass,
}: AvatarProps) {
	const [failed, setFailed] = useState(false);

	// src 变化时重置失败标记，允许换图后重试
	useEffect(() => {
		setFailed(false);
	}, [src]);

	const sizeClass = className ?? "size-10";
	const showImage = Boolean(src) && !failed;

	if (!showImage) {
		return (
			<div
				role="img"
				aria-label={name || alt || "avatar"}
				className={clsx(
					"from-primary to-secondary text-primary-foreground flex shrink-0 select-none items-center justify-center rounded-full bg-gradient-to-br font-bold",
					sizeClass,
					textClass ?? "text-base",
				)}
			>
				{initialOf(name)}
			</div>
		);
	}

	return (
		<div className={clsx("shrink-0 overflow-hidden rounded-full", sizeClass)}>
			<img
				src={src ?? undefined}
				alt={alt ?? name ?? ""}
				onError={() => setFailed(true)}
				draggable={false}
				className="size-full rounded-full object-cover"
			/>
		</div>
	);
}
