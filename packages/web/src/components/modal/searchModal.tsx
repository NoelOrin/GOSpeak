import { createSignal, type Component } from "solid-js";

interface SearchModalProps {
	ref: HTMLDialogElement;
	onClose: () => void;
}

const SearchModal: Component<SearchModalProps> = (props) => {
	// const [searchTerm, setSearchTerm] = createSignal("");
	const handleSearch = (e: Event) => {
		e.preventDefault();
		// // 调用外部传入的搜索处理函数
		// if (props.onSearch) {
		// 	props.onSearch(searchTerm());
		// }
		// 搜索完成后关闭模态框
		// closeSearchModal();
	};
	const [state, setState] = createSignal({
		serverAddress: "",
		serverKey: "",
	});
	const onSubmit = async () => {
		const { serverAddress, serverKey } = state();
		console.log(serverAddress, serverKey);

		if (!serverAddress || !serverKey) {
			return;
		}

		// if(false){
		// 	props.onClose();
		// }
	};

	return (
		<dialog ref={props.ref} class="modal" id="connect_search_modal">
			<div class="modal-box">
				<h3 class="font-bold text-lg">连接服务器</h3>
				<form>
					<div class="space-y-2 pt-2">
						<fieldset class="fieldset">
							<legend class="fieldset-legend">服务器地址</legend>
							<input
								type="text"
								class="w-full input"
								placeholder="输入服务器地址..."
								onInput={(e) =>
									setState({ ...state(), serverAddress: e.target.value })
								}
							/>
						</fieldset>
						<fieldset class="fieldset">
							<legend class="fieldset-legend">服务器连接密钥</legend>
							<input
								type="text"
								class="w-full input"
								placeholder="输入服务器连接密钥..."
								onInput={(e) =>
									setState({ ...state(), serverKey: e.target.value })
								}
							/>
						</fieldset>
					</div>
				</form>
				<form method="dialog" onSubmit={handleSearch} class="modal-backdrop">
					<div class="modal-action">
						<button
							type="button"
							class="btn"
							onClick={() => {
								props.onClose();
							}}
						>
							取消
						</button>
						<button class="btn btn-accent" onClick={onSubmit}>
							搜索
						</button>
					</div>
				</form>
			</div>
		</dialog>
	);
};

export default SearchModal;
