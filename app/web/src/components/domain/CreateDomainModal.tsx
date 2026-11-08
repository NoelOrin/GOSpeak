import {
	Button,
	Checkbox,
	Dialog,
	DialogContent,
	DialogTitle,
	Input,
	toast,
} from "@gospeak/components";
import { useState } from "react";
import { type Domain, createDomain } from "@/api/domain";
import { domainStore } from "@/stores/domainStore";
import { toastActionError } from "@/utils/toast";

export default function CreateDomainModal({
	open,
	onClose,
	onCreated,
}: {
	open: boolean;
	onClose: () => void;
	onCreated?: (domain: Domain) => void;
}) {
	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [isPublic, setIsPublic] = useState(false);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState("");

	const handleCreate = async () => {
		if (!name.trim()) {
			setError("请输入域名称");
			return;
		}
		setLoading(true);
		setError("");
		try {
			const domain = await createDomain({
				name: name.trim(),
				description: description.trim() || undefined,
				is_public: isPublic,
			});
			domainStore.addDomain(domain);
			domainStore.setCurrentDomain(domain.uuid);
			setName("");
			setDescription("");
			setIsPublic(false);
			onClose();
			onCreated?.(domain);
			toast.success(`域「${domain.name}」已创建`);
		} catch (e: any) {
			const message = toastActionError(e, "创建域失败");
			setError(message);
		} finally {
			setLoading(false);
		}
	};

	if (!open) return null;

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent>
				<DialogTitle className="mb-1 text-lg font-bold">创建域</DialogTitle>
				<p className="border-info/30 bg-info/10 text-muted-foreground mb-4 rounded-sm border px-3 py-2 text-xs leading-relaxed">
					域是一个独立的语音空间，类似一个服务器：拥有自己的成员、语音与文字频道，以及独立的权限设置。创建后可以复制邀请链接，把朋友拉进来。
				</p>
				<label className="mb-3 block">
					<span className="mb-1 block text-sm">域名称</span>
					<Input
						type="text"
						value={name}
						onChange={(e) => setName(e.target.value)}
						placeholder="输入域名称"
						autoFocus
						onKeyDown={(e) => {
							// 弹窗内没有 form，回车提交要自己接
							if (e.key === "Enter" && !loading) void handleCreate();
						}}
					/>
				</label>
				<label className="mb-3 block">
					<span className="mb-1 block text-sm">描述（可选）</span>
					<textarea
						className="border-input focus-visible:ring-ring flex w-full rounded-sm border bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2"
						value={description}
						onChange={(e) => setDescription(e.target.value)}
						placeholder="简要描述"
						rows={3}
					/>
				</label>
				<div className="mb-4">
					<label className="flex cursor-pointer items-center gap-2">
						<Checkbox
							checked={isPublic}
							onCheckedChange={(checked) => setIsPublic(checked === true)}
						/>
						<span className="text-sm">公开域</span>
					</label>
					<p className="text-faint-foreground mt-1 pl-6 text-xs">
						公开域会出现在「发现域」列表；关闭后只能凭邀请链接加入。
					</p>
				</div>
				{error && (
					<div className="border-destructive/50 bg-destructive/10 text-destructive mb-3 flex items-center gap-2 rounded-sm px-3 py-2 text-sm">
						<span>{error}</span>
					</div>
				)}
				<div className="flex justify-end gap-2">
					<Button variant="outline" onClick={onClose}>
						取消
					</Button>
					<Button onClick={handleCreate} disabled={loading}>
						{loading ? "创建中..." : "创建"}
					</Button>
				</div>
			</DialogContent>
		</Dialog>
	);
}
