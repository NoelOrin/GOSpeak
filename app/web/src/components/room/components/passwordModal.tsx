import {
	Button,
	Dialog,
	DialogContent,
	DialogTitle,
} from "@gospeak/components";
import { Lock } from "lucide-react";
import { useState } from "react";
import PasswordInput from "@/components/common/passwordInput";
import { chatStore } from "@/stores/chatStore";
import { type RoomInfo, socketStore } from "@/stores/socketStore";

interface PasswordModalProps {
	room: RoomInfo;
	onClose: () => void;
}

export default function PasswordModal({ room, onClose }: PasswordModalProps) {
	const [password, setPassword] = useState("");

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		const pwd = password;
		if (!pwd) return;
		if (room.type === "text") {
			chatStore.joinTextRoom({
				uuid: room.uuid,
				name: room.name,
				password: pwd,
				domain_uuid: room.domain_uuid,
			});
		} else {
			// 设置选择的房间，把密码暂存到 selectedRoomInfo 上供 joinRoom 使用
			socketStore.selectRoom({ ...room, _password: pwd });
		}
		onClose();
	};

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent>
				<DialogTitle className="mb-4 flex items-center gap-1.5 text-lg font-bold">
					<Lock size={20} className="inline-block" />
					房间需要密码
				</DialogTitle>
				<p className="text-muted-foreground mb-4 text-sm">
					房间「{room.name}」需要密码才能加入
				</p>
				<form onSubmit={handleSubmit}>
					<PasswordInput
						autoComplete="off"
						placeholder="请输入房间密码"
						className="mb-4"
						value={password}
						autoFocus
						onChange={(e) => setPassword(e.target.value)}
					/>
					<div className="flex justify-end gap-2">
						<Button type="button" variant="ghost" onClick={onClose}>
							取消
						</Button>
						<Button type="submit">加入</Button>
					</div>
				</form>
			</DialogContent>
		</Dialog>
	);
}
