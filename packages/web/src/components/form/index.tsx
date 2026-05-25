import { createForm } from "@tanstack/solid-form";
import { For, Show, Switch, Match, createEffect } from "solid-js";

export type FormInstanceType = ReturnType<typeof createForm>;

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
  options?: Array<{ value: string; label: string }>;
  validation?: (value: any) => string | undefined;
  className?: string;
}

export type FieldsType = FormFieldConfig<any>[];

export interface FormConfig<TFormData> {
  class: string;
  fields: FormFieldConfig<TFormData>[];
  onSubmit: (values: TFormData) => void;
  submitButtonText?: string;
  showSubmitButton?: boolean;
  formClassName?: string;
  setFormIns?: (form: any) => void;
}

const FormInput = <TFormData,>(props: {
  field: FormFieldConfig<TFormData>;
  form: FormInstanceType;
}) => {
  const { field, form } = props;
  const fieldValue = () => form.getFieldValue(field.name);
  const fieldId = `field-${field.name}`;

  return (
    <Switch>
      <Match when={field.type === "checkbox"}>
        <label class="fieldset-label mb-4 cursor-pointer">
          <input
            id={fieldId}
            type="checkbox"
            checked={(fieldValue() as boolean) || false}
            required={field.required}
            class="checkbox"
            onChange={(e) =>
              form.setFieldValue(field.name, e.target.checked as any)
            }
          />
        </label>
        <Show
          when={
            form.state.fieldMeta[field.name]?.isTouched &&
            (form.state.fieldMeta[field.name]?.errors?.length ?? 0) > 0
          }
        >
          <p class="fieldset-label mb-4 text-error">
            {form.state.fieldMeta[field.name]?.errors?.join(", ")}
          </p>
        </Show>
      </Match>

      <Match when={field.type === "radio"}>
        <fieldset class={`fieldset mb-4 ${field.className || ""}`}>
          <legend class="fieldset-legend">{field.label}</legend>
          <For each={field.options}>
            {(option) => (
              <label class="fieldset-label cursor-pointer">
                <input
                  id={`${fieldId}-${option.value}`}
                  type="radio"
                  name={field.name}
                  value={option.value}
                  checked={fieldValue() === option.value}
                  required={field.required}
                  class="radio"
                  onChange={(e) =>
                    form.setFieldValue(field.name, e.target.value as any)
                  }
                />
                {option.label}
              </label>
            )}
          </For>
          <Show
            when={
              form.state.fieldMeta[field.name]?.isTouched &&
              (form.state.fieldMeta[field.name]?.errors?.length ?? 0) > 0
            }
          >
            <p class="fieldset-label text-error">
              {form.state.fieldMeta[field.name]?.errors?.join(", ")}
            </p>
          </Show>
        </fieldset>
      </Match>

      <Match when={true}>
        <fieldset class={`fieldset mb-4 ${field.className || ""}`}>
          <legend class="fieldset-legend">{field.label}</legend>

          <Switch
            fallback={
              <input
                id={fieldId}
                type={field.type}
                value={(fieldValue() as string) || ""}
                placeholder={field.placeholder}
                required={field.required}
                class="input"
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
                class="textarea h-24"
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
                class="select"
                onChange={(e) =>
                  form.setFieldValue(field.name, e.target.value as any)
                }
              >
                <option value="">请选择</option>
                <For each={field.options}>
                  {(option) => (
                    <option value={option.value}>{option.label}</option>
                  )}
                </For>
              </select>
            </Match>
          </Switch>

          <Show
            when={
              form.state.fieldMeta[field.name]?.isTouched &&
              (form.state.fieldMeta[field.name]?.errors?.length ?? 0) > 0
            }
          >
            <p class="fieldset-label text-error">
              {form.state.fieldMeta[field.name]?.errors?.join(", ")}
            </p>
          </Show>
        </fieldset>
      </Match>
    </Switch>
  );
};

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
          class="btn btn-primary mt-4 w-full"
          disabled={form.state.isSubmitting}
        >
          {props.submitButtonText || "提交"}
        </button>
      </Show>
    </form>
  );
};

export default Form;
