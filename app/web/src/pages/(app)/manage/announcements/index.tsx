import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { Button, Input, Switch, toast } from "@gospeak/components";
import { Megaphone, Trash2 } from "lucide-react";
import { useState } from "react";
import {
	adminListAnnouncements,
	createAnnouncement,
	deleteAnnouncement,
	updateAnnouncement,
} from "@/api/announcement";
import { getMyDomainsDetailed } from "@/api/domain";
import {
	ManageHeader,
	ManageLoading,
	ManagePage,
	ManageSection,
	ManageTag,
	manageTableHeadClass,
	manageTableRowClass,
} from "@/components/manage/ManageShell";
import MarkdownText from "@/components/textRoom/MarkdownText";
import type { AnnouncementDTO } from "@/protocol/announcement";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";

export const Route = createFileRoute("/(app)/manage/announcements/")({
	beforeLoad: () => {
		if (
			!hasPermission("announcement:manage") &&
			!hasPermission("domain:manage")
		) {
			denyAccess();
		}
	},
	component: AnnouncementsPage,
	staticData: { title: "公告" },
});

type AnnouncementLevel = AnnouncementDTO["level"];

interface FormState {
	uuid?: string;
	scopeType: "platform" | "domain";
	scopeId: string;
	title: string;
	content: string;
	level: AnnouncementLevel;
	pinned: boolean;
	publish: boolean;
}

const EMPTY_FORM: FormState = {
	scopeType: "platform",
	scopeId: "",
	title: "",
	content: "",
	level: "info",
	pinned: false,
	publish: true,
};

const LEVEL_LABELS: Record<AnnouncementLevel, string> = {
	info: "普通",
	warning: "提醒",
	critical: "重要",
};

function statusOf(item: AnnouncementDTO): string {
	if (!item.published_at) return "草稿";
	if (item.expires_at && item.expires_at < Date.now()) return "已过期";
	return "已发布";
}

export default function AnnouncementsPage() {
	const queryClient = useQueryClient();
	const isPlatformAdmin = hasPermission("announcement:manage");
	const [form, setForm] = useState<FormState>(EMPTY_FORM);
	const [saving, setSaving] = useState(false);

	const { data, isLoading } = useQuery({
		queryKey: ["manage-announcements"],
		queryFn: () => adminListAnnouncements(),
	});

	const { data: domains = [] } = useQuery({
		queryKey: ["manage-announcement-domains"],
		queryFn: getMyDomainsDetailed,
	});

	const items = data?.items ?? [];
	const scopeChoices = domains as Array<{ uuid: string; name: string }>;

	const reset = () => setForm(EMPTY_FORM);

	const submit = async () => {
		if (!form.title.trim() || !form.content.trim()) {
			toast.error("标题与正文不能为空");
			return;
		}
		if (form.scopeType === "domain" && !form.scopeId) {
			toast.error("请选择要发布公告的域");
			return;
		}
		setSaving(true);
		try {
			const payload = {
				scope_type: form.scopeType,
				scope_id: form.scopeType === "domain" ? form.scopeId : undefined,
				title: form.title.trim(),
				content: form.content,
				level: form.level,
				pinned: form.pinned,
				publish: form.publish,
			};
			if (form.uuid) {
				await updateAnnouncement({ ...payload, uuid: form.uuid });
				toast.success("公告已更新");
			} else {
				await createAnnouncement(payload);
				toast.success(form.publish ? "公告已发布" : "草稿已保存");
			}
			reset();
			await queryClient.invalidateQueries({
				queryKey: ["manage-announcements"],
			});
		} catch (error) {
			toastActionError(error, "保存失败");
		} finally {
			setSaving(false);
		}
	};

	const remove = async (item: AnnouncementDTO) => {
		try {
			await deleteAnnouncement(item.uuid);
			toast.success("公告已删除");
			await queryClient.invalidateQueries({
				queryKey: ["manage-announcements"],
			});
		} catch (error) {
			toastActionError(error, "删除失败");
		}
	};

	const edit = (item: AnnouncementDTO) => {
		setForm({
			uuid: item.uuid,
			scopeType: item.scope_type,
			scopeId: item.scope_id ?? "",
			title: item.title,
			content: item.content,
			level: item.level,
			pinned: item.pinned,
			publish: !!item.published_at,
		});
	};

	return (
		<ManagePage>
			<ManageHeader
				icon={<Megaphone size={20} />}
				title="公告"
				description="发布全站或域内系统公告"
			/>

			<ManageSection title={form.uuid ? "编辑公告" : "新建公告"}>
				<div className="flex flex-col gap-3">
					<div className="flex flex-wrap items-center gap-2 text-sm">
						<span className="text-muted-foreground">范围</span>
						<select
							className="border-border bg-card h-9 rounded-sm border px-2 text-sm"
							value={form.scopeType}
							onChange={(e) =>
								setForm((prev) => ({
									...prev,
									scopeType: e.target.value as "platform" | "domain",
								}))
							}
						>
							{isPlatformAdmin && <option value="platform">全站</option>}
							<option value="domain">指定域</option>
						</select>
						{form.scopeType === "domain" && (
							<select
								className="border-border bg-card h-9 rounded-sm border px-2 text-sm"
								value={form.scopeId}
								onChange={(e) =>
									setForm((prev) => ({ ...prev, scopeId: e.target.value }))
								}
							>
								<option value="">选择域</option>
								{scopeChoices.map((d) => (
									<option key={d.uuid} value={d.uuid}>
										{d.name}
									</option>
								))}
							</select>
						)}
					</div>

					<Input
						placeholder="标题"
						value={form.title}
						onChange={(e) =>
							setForm((prev) => ({ ...prev, title: e.target.value }))
						}
					/>

					<textarea
						className="border-border bg-card min-h-40 rounded-sm border p-3 font-mono text-sm"
						placeholder="正文（支持 Markdown）"
						value={form.content}
						onChange={(e) =>
							setForm((prev) => ({ ...prev, content: e.target.value }))
						}
					/>

					{form.content.trim() && (
						<div className="border-border bg-muted/40 rounded-sm border p-3">
							<div className="text-faint-foreground mb-2 text-2xs">预览</div>
							<MarkdownText text={form.content} />
						</div>
					)}

					<div className="flex flex-wrap items-center gap-4 text-sm">
						<label className="flex items-center gap-2">
							级别
							<select
								className="border-border bg-card h-9 rounded-sm border px-2"
								value={form.level}
								onChange={(e) =>
									setForm((prev) => ({
										...prev,
										level: e.target.value as AnnouncementLevel,
									}))
								}
							>
								{(Object.keys(LEVEL_LABELS) as AnnouncementLevel[]).map((l) => (
									<option key={l} value={l}>
										{LEVEL_LABELS[l]}
									</option>
								))}
							</select>
						</label>
						<label className="flex items-center gap-2">
							<Switch
								checked={form.pinned}
								onCheckedChange={(v) =>
									setForm((prev) => ({ ...prev, pinned: v }))
								}
							/>
							置顶
						</label>
						<label className="flex items-center gap-2">
							<Switch
								checked={form.publish}
								onCheckedChange={(v) =>
									setForm((prev) => ({ ...prev, publish: v }))
								}
							/>
							立即发布
						</label>
					</div>

					<div className="flex items-center gap-2">
						<Button onClick={submit} disabled={saving}>
							{form.uuid ? "保存" : "创建"}
						</Button>
						{form.uuid && (
							<Button variant="ghost" onClick={reset}>
								取消编辑
							</Button>
						)}
					</div>
				</div>
			</ManageSection>

			<ManageSection title="公告列表">
				{isLoading ? (
					<ManageLoading />
				) : items.length === 0 ? (
					<div className="text-faint-foreground py-8 text-center text-sm">
						暂无公告
					</div>
				) : (
					<table className="w-full text-sm">
						<thead className={manageTableHeadClass}>
							<tr>
								<th className="px-2 py-2 text-left">标题</th>
								<th className="px-2 py-2 text-left">范围</th>
								<th className="px-2 py-2 text-left">级别</th>
								<th className="px-2 py-2 text-left">状态</th>
								<th className="px-2 py-2 text-right">操作</th>
							</tr>
						</thead>
						<tbody>
							{items.map((item) => (
								<tr key={item.uuid} className={manageTableRowClass}>
									<td className="max-w-64 truncate px-2 py-2">{item.title}</td>
									<td className="px-2 py-2">
										<ManageTag>
											{item.scope_type === "platform" ? "全站" : "域内"}
										</ManageTag>
									</td>
									<td className="px-2 py-2">{LEVEL_LABELS[item.level]}</td>
									<td className="px-2 py-2">{statusOf(item)}</td>
									<td className="px-2 py-2 text-right whitespace-nowrap">
										<Button
											variant="ghost"
											size="xs"
											onClick={() => edit(item)}
										>
											编辑
										</Button>
										<Button
											variant="ghost"
											size="icon-xs"
											onClick={() => void remove(item)}
											aria-label="删除公告"
										>
											<Trash2 size={14} />
										</Button>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				)}
			</ManageSection>
		</ManagePage>
	);
}
