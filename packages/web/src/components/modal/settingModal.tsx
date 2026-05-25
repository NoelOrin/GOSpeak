import { createEffect, createSignal, type Component } from "solid-js";
import { createForm } from "@tanstack/solid-form";
import Form, { type FormInstanceType, type FieldsType } from "../form";

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
          class="top-2 right-2 absolute border-0 btn btn-sm btn-circle"
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
  return (
    <div class="flex w-full h-full select-none">
      <ul class="bg-base-200 p-0 min-w-32 menu-lg menu join">
        <li class="btn-block btn btn-ghost">Item 1</li>
        <li class="btn-block btn btn-ghost">Item 2</li>
        <li class="btn-block btn btn-ghost">Item 3</li>
      </ul>

      <div class="flex-1">
        <SettingForm />
      </div>
    </div>
  );
};

const SettingForm = () => {
  const settingJson = {
    name: "设置",
    items: [
      {
        name: "房间名称",
        type: "text",
        placeholder: "请输入房间名称",
      },
      {
        name: "房间密码",
        type: "password",
        placeholder: "请输入房间密码",
      },
    ],
  };

  const form = createForm(() => ({
    defaultValues: {
      fullName: "",
    },
    onSubmit: async ({ value }) => {
      // Do something with form data
      console.log(value);
    },
  }));
  const formItems: FieldsType = [
    {
      name: "fullName",
      label: "Full Name",
      type: "text",
      placeholder: "Enter your full name",
    },
    {
      name: "fullNam2e",
      label: "Full Name",
      type: "text",
      placeholder: "Enter your full name",
    },
  ];

  const [formIns, setFormIns] = createSignal<FormInstanceType>();
  createEffect(() => {
    // console.log(formIns());
  });

  return (
    <>
      <Form
        setFormIns={setFormIns}
        class="px-4 py-2"
        fields={formItems}
        onSubmit={form.handleSubmit}
        showSubmitButton={false}
        formClassName="grid grid-cols-2 gap-4 card "
      />
      <button
        type="button"
        onClick={() => {
          console.log(formIns()?.state?.values);
        }}
      >
        提交
      </button>
    </>
  );
};
