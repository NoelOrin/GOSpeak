import {
	RadioGroup,
	RadioGroupItem,
	buttonVariants,
	toast,
} from "@gospeak/components";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { HardDrive } from "lucide-react";
import { useEffect, useState } from "react";
import {
	getStorageConfig,
	type StorageConfigInput,
	testStorageConfig,
	updateStorageConfig,
} from "@/api/storage";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";
import PasswordInput from "@/components/common/passwordInput";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";

export const Route = createFileRoute("/(app)/manage/storage/")({
	beforeLoad: () => {
		if (!hasPermission("storage:read")) {
			denyAccess();
		}
	},
	component: StoragePage,
	staticData: {
		title: "存储",
	},
});

const inputClass =
	"border-input bg-card text-foreground focus-visible:ring-ring placeholder:text-faint-foreground flex h-9 w-full rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2";
const labelClass = "mb-1 block text-sm";
const btn = buttonVariants({ variant: "outline" });

function StoragePage() {
	const [providerType, setProviderType] = useState<"s3" | "local">("local");
	const [endpoint, setEndpoint] = useState("");
	const [bucket, setBucket] = useState("");
	const [region, setRegion] = useState("");
	const [accessKey, setAccessKey] = useState("");
	const [secretKey, setSecretKey] = useState("");
	const [accessKeySet, setAccessKeySet] = useState(false);
	const [secretKeySet, setSecretKeySet] = useState(false);
	const [publicBaseURL, setPublicBaseURL] = useState("");
	const [pathPrefix, setPathPrefix] = useState("uploads/");
	const [maxFileSize, setMaxFileSize] = useState(5);
	const [allowedTypes, setAllowedTypes] = useState(
		"image/jpeg,image/png,image/gif,image/webp",
	);
	const [saving, setSaving] = useState(false);
	const [testing, setTesting] = useState(false);

	const { data: storageConfig, isLoading } = useQuery({
		queryKey: ["storage-config"],
		queryFn: getStorageConfig,
	});

	// 表单由 query 数据回填：原先这里又单独 getStorageConfig() 一次，
	// 同一接口打两个请求，失败时还会弹两条一样的 toast。
	useEffect(() => {
		const cfg = storageConfig;
		if (!cfg) return;
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
	}, [storageConfig]);

	const buildInput = (): StorageConfigInput => {
		const input: StorageConfigInput = {
			provider_type: providerType,
			endpoint,
			bucket,
			region,
			public_base_url: publicBaseURL,
			path_prefix: pathPrefix,
			max_file_size: maxFileSize,
			allowed_types: allowedTypes,
		};
		if (accessKey) input.access_key = accessKey;
		if (secretKey) input.secret_key = secretKey;
		return input;
	};

	const handleSave = async () => {
		if (testing || saving) return;
		setSaving(true);
		try {
			const saved = await updateStorageConfig(buildInput());
			setAccessKeySet(!!saved.access_key_set);
			setSecretKeySet(!!saved.secret_key_set);
			setAccessKey("");
			setSecretKey("");
			toast.success("存储配置已保存");
		} catch (error) {
			toastActionError(error, "保存存储配置失败");
		} finally {
			setSaving(false);
		}
	};

	const handleTestConnection = async () => {
		if (testing || saving) return;
		setTesting(true);
		try {
			await testStorageConfig(buildInput());
			toast.success("连接测试通过");
		} catch (error) {
			toastActionError(error, "连接测试失败");
		} finally {
			setTesting(false);
		}
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<HardDrive size={20} />}
				title="存储"
				description="配置上传后端与访问规则"
			/>

			{isLoading ? (
				<ManageLoading />
			) : (
				<>
					<ManageSection title="存储后端" description="选择对象存储或本地磁盘">
						<div className="flex flex-wrap gap-3">
							<RadioGroup
								value={providerType}
								onValueChange={(value) =>
									setProviderType(value as typeof providerType)
								}
								className="flex flex-wrap gap-3"
							>
								<label className="border-border bg-card flex min-h-11 cursor-pointer items-center gap-2 rounded-lg border px-3 py-2">
									<RadioGroupItem value="s3" />
									<span className="text-sm">S3 兼容存储</span>
								</label>
								<label className="border-border bg-card flex min-h-11 cursor-pointer items-center gap-2 rounded-lg border px-3 py-2">
									<RadioGroupItem value="local" />
									<span className="text-sm">本地存储</span>
								</label>
							</RadioGroup>
						</div>
					</ManageSection>

					{providerType === "s3" && (
						<ManageSection
							title="S3 配置"
							description="密钥留空表示保留已有配置"
						>
							<div className="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
								<label className="block">
									<span className={labelClass}>Endpoint</span>
									<input
										type="text"
										className={inputClass}
										placeholder="https://s3.example.com"
										aria-label="Endpoint"
										value={endpoint}
										onChange={(e) => setEndpoint(e.target.value)}
									/>
								</label>
								<label className="block">
									<span className={labelClass}>Bucket</span>
									<input
										type="text"
										className={inputClass}
										placeholder="my-bucket"
										aria-label="Bucket"
										value={bucket}
										onChange={(e) => setBucket(e.target.value)}
									/>
								</label>
								<label className="block">
									<span className={labelClass}>Region</span>
									<input
										type="text"
										className={inputClass}
										placeholder="us-east-1"
										aria-label="Region"
										value={region}
										onChange={(e) => setRegion(e.target.value)}
									/>
								</label>
								<div />
								<label className="block">
									<span className={labelClass}>Access Key</span>
									<PasswordInput
										native
										autoComplete="off"
										className={inputClass}
										placeholder={
											accessKeySet ? "已配置，留空保留" : "Access Key"
										}
										aria-label="Access Key"
										value={accessKey}
										onChange={(e) => setAccessKey(e.target.value)}
									/>
								</label>
								<label className="block">
									<span className={labelClass}>Secret Key</span>
									<PasswordInput
										native
										autoComplete="off"
										className={inputClass}
										placeholder={
											secretKeySet ? "已配置，留空保留" : "Secret Key"
										}
										aria-label="Secret Key"
										value={secretKey}
										onChange={(e) => setSecretKey(e.target.value)}
									/>
								</label>
							</div>
						</ManageSection>
					)}

					<ManageSection
						title="通用配置"
						description="公开访问地址、路径与上传限制"
						actions={
							<div className="flex gap-2">
								<button
									type="button"
									className={btn}
									disabled={testing || saving}
									onClick={handleTestConnection}
								>
									{testing ? "测试中..." : "测试连接"}
								</button>
								<button
									type="button"
									className={btn}
									disabled={saving || testing}
									onClick={handleSave}
								>
									{saving ? "保存中..." : "保存配置"}
								</button>
							</div>
						}
					>
						<label className="block">
							<span className={labelClass}>公开访问基础 URL（可选）</span>
							<input
								type="text"
								className={inputClass}
								placeholder="https://cdn.example.com（留空则自动拼接）"
								aria-label="公开访问基础 URL"
								value={publicBaseURL}
								onChange={(e) => setPublicBaseURL(e.target.value)}
							/>
							<p className="text-faint-foreground mt-1 text-xs">
								用于 CDN 或自定义域名，留空时 S3 自动拼接、本地用 /uploads 前缀
							</p>
						</label>
						<div className="mt-3 grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
							<label className="block">
								<span className={labelClass}>路径前缀</span>
								<input
									type="text"
									className={inputClass}
									placeholder="uploads/"
									aria-label="路径前缀"
									value={pathPrefix}
									onChange={(e) => setPathPrefix(e.target.value)}
								/>
							</label>
							<label className="block">
								<span className={labelClass}>最大文件大小 (MB)</span>
								<input
									type="number"
									className={inputClass}
									min={1}
									aria-label="最大文件大小 (MB)"
									max={100}
									value={maxFileSize}
									onChange={(e) => setMaxFileSize(Number(e.target.value))}
								/>
							</label>
						</div>
						<label className="mt-3 block">
							<span className={labelClass}>允许的文件类型</span>
							<textarea
								className={`${inputClass} h-auto py-2`}
								rows={2}
								aria-label="允许的文件类型"
								placeholder="image/jpeg,image/png,image/gif,image/webp"
								value={allowedTypes}
								onChange={(e) => setAllowedTypes(e.target.value)}
							/>
							<p className="text-faint-foreground mt-1 text-xs">
								逗号分隔 MIME 类型
							</p>
						</label>
					</ManageSection>
				</>
			)}
		</ManagePage>
	);
}
