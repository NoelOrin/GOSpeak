import { For, Show, Switch, Match } from "solid-js";

export interface FormFieldConfig {
  label: string;
  name: string;
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

export type FieldsType = FormFieldConfig[];

export interface FormProps {
  form: any;
  fields: FieldsType;
  class?: string;
  formClassName?: string;
  showSubmitButton?: boolean;
  submitButtonText?: string;
}

const FormInput = (props: {
  field: FormFieldConfig;
  form: any;
}) => {
  const { field, form } = props;
  const fieldValue = () => form.getFieldValue(field.name);
  const fieldErrors = () => form.state.fieldMeta[field.name]?.errors ?? [];
  const isTouched = () => form.state.fieldMeta[field.name]?.isTouched;
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
        <Show when={isTouched() && fieldErrors().length > 0}>
          <p class="fieldset-label mb-4 text-error">
            {fieldErrors().join(", ")}
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
          <Show when={isTouched() && fieldErrors().length > 0}>
            <p class="fieldset-label text-error">
              {fieldErrors().join(", ")}
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

          <Show when={isTouched() && fieldErrors().length > 0}>
            <p class="fieldset-label text-error">
              {fieldErrors().join(", ")}
            </p>
          </Show>
        </fieldset>
      </Match>
    </Switch>
  );
};

export const Form = (props: FormProps) => {
  const form = props.form;

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
          {(config) => (
            <form.Field
              name={config.name}
              validators={{
                onChange: ({ value }: { value: any }) => {
                  if (config.required && (!value || value === "")) {
                    return `${config.label} 是必填项`;
                  }
                  if (config.validation) {
                    return config.validation(value);
                  }
                },
              }}
            >
              {(_field: any) => (
                <FormInput field={config} form={form} />
              )}
            </form.Field>
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