/** 系统公告 DTO — 与后端 service.AnnouncementDTO 对齐 */
export interface AnnouncementDTO {
	uuid: string;
	scope_type: "platform" | "domain";
	scope_id?: string;
	title: string;
	content: string;
	level: "info" | "warning" | "critical";
	pinned: boolean;
	published_at?: number;
	expires_at?: number;
	created_by_uuid?: string;
	created_by_name: string;
	is_read: boolean;
	created_at: number;
}

export interface AnnouncementListResult {
	items: AnnouncementDTO[];
	total: number;
}

export interface AnnouncementInput {
	uuid?: string;
	scope_type: "platform" | "domain";
	scope_id?: string;
	title: string;
	content: string;
	level: "info" | "warning" | "critical";
	pinned?: boolean;
	publish?: boolean;
	expires_at?: number | null;
}
