import { For, Show, createSignal, createResource, createEffect, type Component } from "solid-js";
import Form, { type FormInstanceType } from "../../form";
import TABS, { type SettingTabConfig } from "./tab_item";
import { getAudioDevices } from "../../../hooks/media";
import { buildAudioFields, getDefaultDeviceValues } from "./tab_item/audio";

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
        <SettingForm tab={TABS[activeTab()]} />
      </div>
    </div>
  );
};

const SettingForm = (props: { tab: SettingTabConfig }) => {
  const [formIns, setFormIns] = createSignal<FormInstanceType>();
  const isAudioTab = () => props.tab.label === "音频";

  const [audioDevices] = createResource(isAudioTab, async () => {
    if (!isAudioTab()) return null;
    return getAudioDevices();
  });

  const fields = () => {
    if (isAudioTab() && audioDevices()) {
      return buildAudioFields(audioDevices()!);
    }
    return props.tab.fields;
  };

  createEffect(() => {
    const devices = audioDevices();
    const f = formIns();
    if (isAudioTab() && devices && f) {
      const defaults = getDefaultDeviceValues(devices);
      for (const [key, value] of Object.entries(defaults)) {
        f.setFieldValue(key, value as any);
      }
    }
  });

  return (
    <div class="p-4">
      <h3 class="mb-4 font-bold text-lg">{props.tab.label}</h3>
      <Show when={!isAudioTab() || audioDevices()} fallback={
        <div class="flex justify-center items-center py-12">
          <span class="loading loading-spinner loading-lg" />
        </div>
      }>
        <Form
          setFormIns={setFormIns}
          class="px-4 py-2"
          fields={fields()}
          onSubmit={props.tab.onSubmit}
          showSubmitButton
          submitButtonText="保存"
          formClassName="grid grid-cols-2 gap-4 card"
        />
      </Show>
    </div>
  );
};