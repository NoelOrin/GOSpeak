import { createForm } from "@tanstack/solid-form";
import { For, Show, Switch, Match, createEffect } from "solid-js";

export type FormInstanceType = ReturnType<typeof createForm>;

// 定义表单字段配置类型
export interface FormFieldConfig<TFormData> {
  label: string;
  name: keyof TFormData & string;
  type:
    | "text"
    | "email"
    | "password"
    | "number"
    | "select"
    | "checkbox"
    | "textarea"
    | "radio";
  placeholder?: string;
  required?: boolean;
  options?: Array<{ value: string; label: string }>; // 用于 select 和 radio
  validation?: (value: any) => string | undefined;
  className?: string;
}

export type FieldsType = FormFieldConfig<any>[];

// 表单配置接口
export interface FormConfig<TFormData> {
  class: string;
  fields: FormFieldConfig<TFormData>[];
  onSubmit: (values: TFormData) => void;
  submitButtonText?: string;
  showSubmitButton?: boolean;
  formClassName?: string;
  setFormIns?: (form: any) => void;
}

// Input 组件
const FormInput = <TFormData,>(props: {
  field: FormFieldConfig<TFormData>;
  form: FormInstanceType;
}) => {
  const { field, form } = props;
  const fieldValue = () => form.getFieldValue(field.name);

  // 生成唯一的ID用于label关联
  const fieldId = `field-${field.name}`;

  return (
    <div class={`mb-4 ${field.className || ""}`}>
      <label for={fieldId} class="block mb-1 font-medium text-sm">
        {field.label}
      </label>

      <Switch
        fallback={
          <input
            id={fieldId}
            type={field.type}
            value={(fieldValue() as string) || ""}
            placeholder={field.placeholder}
            required={field.required}
            class="px-3 py-2 border rounded-md w-full"
            // onBlur={() => form.setFieldTouched(field.name, true)}
            onInput={(e) => {
              const value =
                field.type === "number"
                  ? (e.target as HTMLInputElement).valueAsNumber
                  : (e.target as HTMLInputElement).value;
              form.setFieldValue(field.name, value as any);
            }}
          />
        }
      >
        <Match when={field.type === "textarea"}>
          <textarea
            id={fieldId}
            value={(fieldValue() as string) || ""}
            placeholder={field.placeholder}
            required={field.required}
            class="px-3 py-2 border rounded-md w-full"
            // onBlur={() => form.setFieldTouched(field.name, true)}
            onInput={(e) =>
              form.setFieldValue(field.name, e.target.value as any)
            }
          />
        </Match>

        <Match when={field.type === "select"}>
          <select
            id={fieldId}
            value={(fieldValue() as string) || ""}
            required={field.required}
            class="px-3 py-2 border rounded-md w-full"
            // onBlur={() => form.setFieldTouched(field.name, true)}
            onChange={(e) =>
              form.setFieldValue(field.name, e.target.value as any)
            }
          >
            <option value="">请选择</option>
            <For each={field.options}>
              {(option) => <option value={option.value}>{option.label}</option>}
            </For>
          </select>
        </Match>

        <Match when={field.type === "checkbox"}>
          <input
            id={fieldId}
            type="checkbox"
            checked={(fieldValue() as boolean) || false}
            required={field.required}
            class="w-4 h-4"
            // onBlur={() => form.setFieldTouched(field.name, true)}
            onChange={(e) =>
              form.setFieldValue(field.name, e.target.checked as any)
            }
          />
        </Match>

        <Match when={field.type === "radio"}>
          <div class="space-y-2">
            <For each={field.options}>
              {(option) => (
                <label class="flex items-center">
                  <input
                    id={`${fieldId}-${option.value}`}
                    type="radio"
                    name={field.name}
                    value={option.value}
                    checked={fieldValue() === option.value}
                    required={field.required}
                    class="mr-2"
                    // onBlur={() => form.setFieldTouched(field.name, true)}
                    onChange={(e) =>
                      form.setFieldValue(field.name, e.target.value as any)
                    }
                  />
                  {option.label}
                </label>
              )}
            </For>
          </div>
        </Match>
      </Switch>

      <Show
        when={
          form.state.fieldMeta[field.name]?.isTouched &&
          (form.state.fieldMeta[field.name]?.errors?.length ?? 0) > 0
        }
      >
        <div class="mt-1 text-red-500 text-sm">
          {form.state.fieldMeta[field.name]?.errors?.join(", ")}
        </div>
      </Show>
    </div>
  );
};

// 动态表单组件
export const Form = <TFormData extends Record<string, any>>(
  props: FormConfig<TFormData>
) => {
  const form = createForm(() => ({
    defaultValues: {} as TFormData,
    onSubmit: ({ value }) => props.onSubmit(value),
    validators: {
      onChange: ({ value }) => {
        const errors: string[] = [];
        for (const field of props.fields) {
          if (
            field.required &&
            (!value[field.name] || value[field.name] === "")
          ) {
            errors.push(`${field.label} 是必填项`);
          }
          if (field.validation) {
            const error = field.validation(value[field.name]);
            if (error) errors.push(error);
          }
        }
        return errors.length > 0 ? errors.join(", ") : undefined;
      },
    },
  }));

  createEffect(() => {
    props.setFormIns?.(form);
  });

  return (
    <form
      class={props.class || ""}
      onSubmit={(e) => {
        e.preventDefault();
        e.stopPropagation();
        form.handleSubmit();
      }}
    >
      <div class={props.formClassName || ""}>
        <For each={props.fields}>
          {(formField) => (
            <form.Field
              name={formField.name as keyof TFormData & string}
              children={(field) => (
                <FormInput field={formField} form={form as any} />
              )}
            />
          )}
        </For>
      </div>

      <Show
        when={
          props.showSubmitButton !== undefined ? props.showSubmitButton : true
        }
      >
        <button
          type="submit"
          class="bg-blue-500 hover:bg-blue-600 px-4 py-2 rounded-md w-full text-white"
          disabled={form.state.isSubmitting}
        >
          {props.submitButtonText || "提交"}
        </button>
      </Show>
    </form>
  );
};

export default Form;
