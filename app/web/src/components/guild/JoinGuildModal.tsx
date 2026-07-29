import { type Component, createSignal } from "solid-js";
import { joinGuild } from "@/api/guild";
import guildStore from "@/stores/guildStore";

const JoinGuildModal: Component<{
	ref: HTMLDialogElement | ((el: HTMLDialogElement) => void);
	onClose: () => void;
}> = (props) => {
	const [inviteCode, setInviteCode] = createSignal("");
	const [loading, setLoading] = createSignal(false);
	const [error, setError] = createSignal("");

	const handleJoin = async () => {
		if (!inviteCode().trim()) {
			setError("请输入邀请码");
			return;
		}
		setLoading(true);
		setError("");
		try {
			const guild = await joinGuild(inviteCode().trim());
			guildStore.addGuild(guild);
			guildStore.setCurrentGuild(guild.uuid);
			props.onClose();
		} catch (e: any) {
			setError(e?.response?.data?.msg || "加入失败");
		} finally {
			setLoading(false);
		}
	};

	return (
		<dialog ref={props.ref} class="modal" onClose={props.onClose}>
			<div class="modal-box">
				<h3 class="font-bold text-lg mb-4">加入语音服务器</h3>
				<div class="form-control mb-3">
					<label class="label" for="guild-join-code">
						<span class="label-text">邀请码</span>
					</label>
					<input
						id="guild-join-code"
						type="text"
						class="input input-bordered"
						value={inviteCode()}
						onInput={(e) => setInviteCode(e.currentTarget.value)}
						placeholder="输入邀请码"
					/>
				</div>
				{error() && (
					<div class="alert alert-error mb-3">
						<span>{error()}</span>
					</div>
				)}
				<div class="modal-action">
					<button class="btn" onClick={props.onClose}>
						取消
					</button>
					<button
						class="btn btn-primary"
						onClick={handleJoin}
						disabled={loading()}
					>
						{loading() ? "加入中..." : "加入"}
					</button>
				</div>
			</div>
		</dialog>
	);
};

export default JoinGuildModal;
