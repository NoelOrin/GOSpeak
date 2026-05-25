import { For, createSignal, type Component } from "solid-js";
import { Dynamic } from "solid-js/web";
import TABS from "./tab_item";

interface SearchModalProps {
  ref: HTMLDialogElement;
  onClose: () => void;
}

const SettingModal: Component<SearchModalProps> = ({ ...props }) => {
  props.ref?.showModal?.();

  return (
    <dialog ref={props.ref} class="modal">
      <div class="p-0 sm:w-full lg:w-[90%] sm:max-w-[100vw] lg:max-w-8xl sm:h-full lg:h-[90%] modal-box">
        <button
          class="top-2 right-2 absolute border-0 z-10 btn btn-sm btn-circle"
          onClick={props.onClose}
        >
          ✕
        </button>
        <SettingContext />
      </div>

      <form method="dialog" class="modal-backdrop">
        <button></button>
      </form>
    </dialog>
  );
};

export default SettingModal;

const SettingContext = () => {
  const [activeTab, setActiveTab] = createSignal(0)

  return (
    <div class="flex w-full h-full select-none">
      <ul class="bg-base-200 p-0 min-w-40 menu-lg menu join">
        <For each={TABS}>
          {(tab, index) => (
            <li
              class="btn-block btn btn-ghost py-6! text-base"
              classList={{
                "bg-base-300": activeTab() === index(),
              }}
              onClick={() => setActiveTab(index())}
            >
              {tab.label}
            </li>
          )}
        </For>
      </ul>

      <div class="flex-1">
        <Dynamic component={TABS[activeTab()].component} />
      </div>
    </div>
  );
};