import { type JSX, type Component } from "solid-js";

interface SearchModalProps {
    children: JSX.Element;
    ref: HTMLDialogElement;
    onClose: () => void;
}

const SearchModal: Component<SearchModalProps> = ({ children,...props}) => {
   

    return (
        <dialog ref={props.ref} class="modal" id="connect_search_modal">
            <div class="modal-box">
              { children }
                <form method="dialog" class="modal-backdrop">
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
                        <button class="btn btn-accent">
                            搜索
                        </button>
                    </div>
                </form>
            </div>
        </dialog>
    );
};

export default SearchModal;
