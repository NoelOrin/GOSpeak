import { createFileRoute, redirect } from "@tanstack/solid-router";
import userStore from "@/stores/userStore";
import { createSignal, onMount, Show } from "solid-js";
import { getStorageConfig, updateStorageConfig, type StorageConfigInput } from "@/api/storage";
import { showToast } from "solid-notifications";
import HardDrive from "lucide-solid/icons/hard-drive";

export const Route = createFileRoute("/(app)/manage/storage/")({
	beforeLoad: () => {
		if (userStore.user()?.role !== "admin") {
			throw redirect({ to: "/" });
		}
	},
	component: StoragePage,
	staticData: {
		title: "存储",
		icon: "icon-manage",
	},
});

function StoragePage() {
	const [providerType, setProviderType] = createSignal<"s3" | "local">("local");
	const [endpoint, setEndpoint] = createSignal("");
	const [bucket, setBucket] = createSignal("");
	const [region, setRegion] = createSignal("");
	const [accessKey, setAccessKey] = createSignal("");
	const [secretKey, setSecretKey] = createSignal("");
	const [publicBaseURL, setPublicBaseURL] = createSignal("");
	const [pathPrefix, setPathPrefix] = createSignal("uploads/");
	const [maxFileSize, setMaxFileSize] = createSignal(5);
	const [allowedTypes, setAllowedTypes] = createSignal("image/jpeg,image/png,image/gif,image/webp");
	const [saving, setSaving] = createSignal(false);
	const [testing, setTesting] = createSignal(false);
	const [loading, setLoading] = createSignal(true);

	onMount(async () => {
		try {
			const cfg = await getStorageConfig();
			setProviderType(cfg.provider_type);
			setEndpoint(cfg.endpoint);
			setBucket(cfg.bucket);
			setRegion(cfg.region);
			setPublicBaseURL(cfg.public_base_url);
			setPathPrefix(cfg.path_prefix);
			setMaxFileSize(cfg.max_file_size);
			setAllowedTypes(cfg.allowed_types);
		} catch (e) {
			const msg = e instanceof Error ? e.message : "加载配置失败";
			showToast(msg, { type: "error" });
		} finally {
			setLoading(false);
		}
	});

	const handleSave = async () => {
		setSaving(true);
		try {
			const input: StorageConfigInput = {
				provider_type: providerType(),
				endpoint: endpoint(),
				bucket: bucket(),
				region: region(),
				public_base_url: publicBaseURL(),
				path_prefix: pathPrefix(),
				max_file_size: maxFileSize(),
				allowed_types: allowedTypes(),
			};
			if (accessKey()) input.access_key = accessKey();
			if (secretKey()) input.secret_key = secretKey();

			await updateStorageConfig(input);
			showToast("存储配置已保存", { type: "success" });
			setAccessKey("");
			setSecretKey("");
		} catch (e) {
			const msg = e instanceof Error ? e.message : "保存失败";
			showToast(msg, { type: "error" });
		} finally {
			setSaving(false);
		}
	};

	const handleTestConnection = async () => {
		setTesting(true);
		try {
			const input: StorageConfigInput = {
				provider_type: providerType(),
				endpoint: endpoint(),
				bucket: bucket(),
				region: region(),
				public_base_url: publicBaseURL(),
				path_prefix: pathPrefix(),
				max_file_size: maxFileSize(),
				allowed_types: allowedTypes(),
			};
			if (accessKey()) input.access_key = accessKey();
			if (secretKey()) input.secret_key = secretKey();

			await updateStorageConfig(input);

			if (providerType() === "s3") {
				const { presignUpload } = await import("@/api/storage");
				await presignUpload({
					file_name: "test.txt",
					content_type: "text/plain",
					file_size: 1,
					category: "test",
				});
			}

			showToast("连接测试成功", { type: "success" });
		} catch (e) {
			const msg = e instanceof Error ? e.message : "连接测试失败";
			showToast(msg, { type: "error" });
		} finally {
			setTesting(false);
		}
	};

	return (
		<Show when={!loading()} fallback={<span class="loading loading-spinner loading-lg" />}>
			<div class="p-4 flex flex-col gap-4">
				<div class="flex items-center gap-2">
					<HardDrive size={20} />
					<h3 class="font-bold text-lg">存储</h3>
				</div>

				<div class="divider my-0 text-xs text-base-content/40">存储后端</div>
				<div class="flex gap-4">
					<label class="flex items-center gap-2 cursor-pointer">
						<input
							type="radio"
							name="storage_provider"
							class="radio radio-sm radio-primary"
							checked={providerType() === "s3"}
							onChange={() => setProviderType("s3")}
						/>
						<span class="text-sm">S3 兼容存储</span>
					</label>
					<label class="flex items-center gap-2 cursor-pointer">
						<input
							type="radio"
							name="storage_provider"
							class="radio radio-sm radio-primary"
							checked={providerType() === "local"}
							onChange={() => setProviderType("local")}
						/>
						<span class="text-sm">本地存储</span>
					</label>
				</div>

				<Show when={providerType() === "s3"}>
					<div class="divider my-0 text-xs text-base-content/40">S3 配置</div>
					<div class="grid grid-cols-2 gap-3">
						<fieldset class="fieldset">
							<legend class="fieldset-legend text-[14px]">Endpoint</legend>
							<input
								type="text"
								class="input input-bordered input-sm w-full"
								placeholder="https://s3.example.com"
								value={endpoint()}
								onInput={(e) => setEndpoint(e.target.value)}
							/>
						</fieldset>
						<fieldset class="fieldset">
							<legend class="fieldset-legend text-[14px]">Bucket</legend>
							<input
								type="text"
								class="input input-bordered input-sm w-full"
								placeholder="my-bucket"
								value={bucket()}
								onInput={(e) => setBucket(e.target.value)}
							/>
						</fieldset>
						<fieldset class="fieldset">
							<legend class="fieldset-legend text-[14px]">Region</legend>
							<input
								type="text"
								class="input input-bordered input-sm w-full"
								placeholder="us-east-1"
								value={region()}
								onInput={(e) => setRegion(e.target.value)}
							/>
						</fieldset>
						<div />
						<fieldset class="fieldset">
							<legend class="fieldset-legend text-[14px]">Access Key</legend>
							<input
								type="password"
								class="input input-bordered input-sm w-full"
								placeholder="留空保持原值"
								value={accessKey()}
								onInput={(e) => setAccessKey(e.target.value)}
							/>
						</fieldset>
						<fieldset class="fieldset">
							<legend class="fieldset-legend text-[14px]">Secret Key</legend>
							<input
								type="password"
								class="input input-bordered input-sm w-full"
								placeholder="留空保持原值"
								value={secretKey()}
								onInput={(e) => setSecretKey(e.target.value)}
							/>
						</fieldset>
					</div>
				</Show>

				<div class="divider my-0 text-xs text-base-content/40">通用配置</div>
				<fieldset class="fieldset">
					<legend class="fieldset-legend text-[14px]">公开访问基础 URL（可选）</legend>
					<input
						type="text"
						class="input input-bordered input-sm w-full"
						placeholder="https://cdn.example.com（留空则自动拼接）"
						value={publicBaseURL()}
						onInput={(e) => setPublicBaseURL(e.target.value)}
					/>
					<p class="label text-[12px] text-base-content/50">用于 CDN 或自定义域名，留空时 S3 自动拼接、本地用 /uploads 前缀</p>
				</fieldset>
				<div class="grid grid-cols-2 gap-3">
					<fieldset class="fieldset">
						<legend class="fieldset-legend text-[14px]">路径前缀</legend>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="uploads/"
							value={pathPrefix()}
							onInput={(e) => setPathPrefix(e.target.value)}
						/>
					</fieldset>
					<fieldset class="fieldset">
						<legend class="fieldset-legend text-[14px]">最大文件大小 (MB)</legend>
						<input
							type="number"
							class="input input-bordered input-sm w-full"
							min="1"
							max="100"
							value={maxFileSize()}
							onInput={(e) => setMaxFileSize(Number(e.target.value))}
						/>
					</fieldset>
				</div>
				<fieldset class="fieldset">
					<legend class="fieldset-legend text-[14px]">允许的文件类型</legend>
					<textarea
						class="textarea textarea-bordered textarea-sm w-full"
						rows="2"
						placeholder="image/jpeg,image/png,image/gif,image/webp"
						value={allowedTypes()}
						onInput={(e) => setAllowedTypes(e.target.value)}
					/>
					<p class="label text-[12px] text-base-content/50">逗号分隔 MIME 类型</p>
				</fieldset>

				<div class="flex gap-3 mt-2">
					<button
						class="btn btn-primary btn-sm"
						classList={{ "btn-disabled": saving() }}
						onClick={handleSave}
					>
						<Show when={saving()} fallback="保存配置">
							<span class="loading loading-spinner loading-xs" /> 保存中...
						</Show>
					</button>
					<Show when={providerType() === "s3"}>
						<button
							class="btn btn-outline btn-sm"
							classList={{ "btn-disabled": testing() }}
							onClick={handleTestConnection}
						>
							<Show when={testing()} fallback="测试连接">
								<span class="loading loading-spinner loading-xs" /> 测试中...
							</Show>
						</button>
					</Show>
				</div>
			</div>
		</Show>
	);
}
