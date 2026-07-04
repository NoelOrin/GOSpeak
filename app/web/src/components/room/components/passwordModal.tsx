import { createSignal, Show } from "solid-js";
import { type RoomInfo, socketStore } from "@/stores/socketStore";

interface PasswordModalProps {
	room: RoomInfo;
	onClose: () => void;
}

const PasswordModal = ({ room, onClose }: PasswordModalProps) => {
	const [password, setPassword] = createSignal("");

	const handleSubmit = (e: Event) => {
		e.preventDefault();
		const pwd = password();
		if (!pwd) return;
		// 设置选择的房间，把密码暂存到 selectedRoomInfo 上供 joinRoom 使用
		socketStore.selectRoom({ ...room, _password: pwd });
		onClose();
	};

	return (
		<dialog class="modal modal-open">
			<div class="modal-box">
				<h3 class="font-bold text-lg mb-4">🔒 房间需要密码</h3>
				<p class="text-sm text-base-content/60 mb-4">
					房间「{room.name}」需要密码才能加入
				</p>
				<form onSubmit={handleSubmit}>
					<input
						type="password"
						placeholder="请输入房间密码"
						class="input input-bordered w-full mb-4"
						value={password()}
						onInput={(e) => setPassword(e.currentTarget.value)}
						autofocus
					/>
					<div class="modal-action">
						<button type="button" class="btn btn-ghost" onClick={onClose}>
							取消
						</button>
						<button type="submit" class="btn btn-primary">
							加入
						</button>
					</div>
				</form>
			</div>
			<form method="dialog" class="modal-backdrop">
				<button onClick={onClose}>close</button>
			</form>
		</dialog>
	);
};

export default PasswordModal;
