import { createSignal } from "solid-js";
import { confirmUpload, presignUpload, uploadFile } from "@/api/storage";

export interface UploadResult {
	public_url: string;
	object_key: string;
}

/**
 * useUpload — 统一上传 hook
 * S3 模式: presign → PUT 直传 → confirm
 * Local 模式: presign(拿 object_key) → POST upload
 */
export function useUpload(category: string) {
	const [uploading, setUploading] = createSignal(false);
	const [error, setError] = createSignal<string | null>(null);
	const [progress, setProgress] = createSignal(0);

	async function upload(file: File): Promise<UploadResult> {
		setUploading(true);
		setError(null);
		setProgress(0);

		try {
			// 1. 获取预签名 / object_key
			const presignResult = await presignUpload({
				file_name: file.name,
				content_type: file.type,
				file_size: file.size,
				category,
			});

			setProgress(20);

			let publicUrl: string;

			if (presignResult.provider_type === "s3" && presignResult.upload_url) {
				// 2a. S3: 直传到预签名 URL
				const putRes = await fetch(presignResult.upload_url, {
					method: "PUT",
					body: file,
					headers: { "Content-Type": file.type },
				});

				setProgress(70);

				if (!putRes.ok) {
					throw new Error(
						`S3 upload failed: ${putRes.status} ${putRes.statusText}`,
					);
				}

				// 3. 确认上传
				const confirmResult = await confirmUpload(presignResult.object_key);
				publicUrl = confirmResult.public_url;
			} else {
				// 2b. Local: 服务器中转上传
				const uploadResult = await uploadFile(file, presignResult.object_key);
				publicUrl = uploadResult.public_url;
			}

			setProgress(100);
			return { public_url: publicUrl, object_key: presignResult.object_key };
		} catch (e) {
			const msg = e instanceof Error ? e.message : "上传失败";
			setError(msg);
			throw e;
		} finally {
			setUploading(false);
		}
	}

	return { upload, uploading, error, progress };
}
