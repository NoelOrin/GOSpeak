import { createFileRoute, redirect } from "@tanstack/solid-router";
import HardDrive from "lucide-solid/icons/hard-drive";
import { createSignal, onMount, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	getStorageConfig,
	type StorageConfigInput,
	updateStorageConfig,
} from "@/api/storage";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import { hasPermission } from "@/utils/permissions";

export const Route = createFileRoute("/(app)/manage/storage/")({
	beforeLoad: () => {
		if (!hasPermission("storage:read")) {
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
	const [accessKeySet, setAccessKeySet] = createSignal(false);
	const [secretKeySet, setSecretKeySet] = createSignal(false);
	const [publicBaseURL, setPublicBaseURL] = createSignal("");
	const [pathPrefix, setPathPrefix] = createSignal("uploads/");
	const [maxFileSize, setMaxFileSize] = createSignal(5);
	const [allowedTypes, setAllowedTypes] = createSignal(
		"image/jpeg,image/png,image/gif,image/webp",
	);
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
			setAccessKeySet(!!cfg.access_key_set);
			setSecretKeySet(!!cfg.secret_key_set);
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

			const saved = await updateStorageConfig(input);
			setAccessKeySet(!!saved.access_key_set);
			setSecretKeySet(!!saved.secret_key_set);
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
		<Show
			when={!loading()}
			fallback={
				<div class="flex h-full min-h-52 items-center justify-center">
					<span class="loading loading-spinner loading-lg" />
				</div>
			}
		>
			<ManagePage>
				<ManageHeader
					icon={<HardDrive size={18} />}
					title="存储"
					description="配置上传后端与访问规则"
				/>

				<ManageSection title="存储后端" description="选择对象存储或本地磁盘">
					<div class="flex flex-wrap gap-3">
						<label class="flex cursor-pointer items-center gap-2 rounded-xl border border-base-300 bg-base-100 px-3 py-2">
							<input
								type="radio"
								name="storage_provider"
								class="radio radio-sm"
								checked={providerType() === "s3"}
								onChange={() => setProviderType("s3")}
							/>
							<span class="text-sm">S3 兼容存储</span>
						</label>
						<label class="flex cursor-pointer items-center gap-2 rounded-xl border border-base-300 bg-base-100 px-3 py-2">
							<input
								type="radio"
								name="storage_provider"
								class="radio radio-sm"
								checked={providerType() === "local"}
								onChange={() => setProviderType("local")}
							/>
							<span class="text-sm">本地存储</span>
						</label>
					</div>
				</ManageSection>

				<Show when={providerType() === "s3"}>
					<ManageSection title="S3 配置" description="密钥留空表示保留已有配置">
						<div class="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
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
									placeholder={
										accessKeySet() ? "已配置，留空保留" : "Access Key"
									}
									value={accessKey()}
									onInput={(e) => setAccessKey(e.target.value)}
								/>
							</fieldset>
							<fieldset class="fieldset">
								<legend class="fieldset-legend text-[14px]">Secret Key</legend>
								<input
									type="password"
									class="input input-bordered input-sm w-full"
									placeholder={
										secretKeySet() ? "已配置，留空保留" : "Secret Key"
									}
									value={secretKey()}
									onInput={(e) => setSecretKey(e.target.value)}
								/>
							</fieldset>
						</div>
					</ManageSection>
				</Show>

				<ManageSection
					title="通用配置"
					description="公开访问地址、路径与上传限制"
					actions={
						<div class="flex gap-2">
							<button
								class="btn btn-sm border border-base-300 bg-base-100 text-base-content/80 shadow-none hover:bg-base-200"
								classList={{ "btn-disabled": testing() || saving() }}
								onClick={handleTestConnection}
							>
								{testing() ? "测试中..." : "测试连接"}
							</button>
							<button
								class="btn btn-sm border border-base-300 bg-base-100 text-base-content shadow-none hover:bg-base-200"
								classList={{ "btn-disabled": saving() || testing() }}
								onClick={handleSave}
							>
								{saving() ? "保存中..." : "保存配置"}
							</button>
						</div>
					}
				>
					<fieldset class="fieldset">
						<legend class="fieldset-legend text-[14px]">
							公开访问基础 URL（可选）
						</legend>
						<input
							type="text"
							class="input input-bordered input-sm w-full"
							placeholder="https://cdn.example.com（留空则自动拼接）"
							value={publicBaseURL()}
							onInput={(e) => setPublicBaseURL(e.target.value)}
						/>
						<p class="label text-[12px] text-base-content/50">
							用于 CDN 或自定义域名，留空时 S3 自动拼接、本地用 /uploads 前缀
						</p>
					</fieldset>
					<div class="mt-3 grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
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
							<legend class="fieldset-legend text-[14px]">
								最大文件大小 (MB)
							</legend>
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
					<fieldset class="fieldset mt-3">
						<legend class="fieldset-legend text-[14px]">允许的文件类型</legend>
						<textarea
							class="textarea textarea-bordered textarea-sm w-full"
							rows="2"
							placeholder="image/jpeg,image/png,image/gif,image/webp"
							value={allowedTypes()}
							onInput={(e) => setAllowedTypes(e.target.value)}
						/>
						<p class="label text-[12px] text-base-content/50">
							逗号分隔 MIME 类型
						</p>
					</fieldset>
				</ManageSection>
			</ManagePage>
		</Show>
	);
}
