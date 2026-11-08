import {
	Button,
	Checkbox,
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
	Switch,
} from "@gospeak/components";
import clsx from "clsx";
import { useState } from "react";
import type { FormEvent } from "react";

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
		| "radio"
		| "switch";
	placeholder?: string;
	required?: boolean;
	options?: Array<{ value: string; label: string }>;
	validation?: (value: any) => string | undefined;
	className?: string;
}

export type FieldsType = FormFieldConfig[];

export interface FormProps {
	fields: FieldsType;
	defaultValues?: Record<string, any>;
	formClassName?: string;
	showSubmitButton?: boolean;
	submitButtonText?: string;
	onSubmit: (values: Record<string, any>) => void | Promise<void>;
}

const inputClass =
	"border-input focus-visible:ring-ring flex h-10 w-full rounded-sm border bg-transparent px-3 py-1 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50";

// Radix Select 不允许空字符串作为 value，用哨兵值映射业务层的空值
const EMPTY_VALUE = "__empty__";

function Field({
	field,
	value,
	error,
	touched,
	onChange,
	onBlur,
}: {
	field: FormFieldConfig;
	value: any;
	error?: string;
	touched: boolean;
	onChange: (v: any) => void;
	onBlur: () => void;
}) {
	const fieldId = `field-${field.name}`;
	const showError = touched && !!error;

	const body = (() => {
		switch (field.type) {
			case "switch":
			case "checkbox":
				return (
					<label
						htmlFor={fieldId}
						className="flex h-full cursor-pointer items-center gap-2"
					>
						{field.type === "switch" ? (
							<Switch
								id={fieldId}
								checked={value ?? false}
								onCheckedChange={onChange}
							/>
						) : (
							<Checkbox
								id={fieldId}
								checked={value ?? false}
								onCheckedChange={(checked) => onChange(checked === true)}
							/>
						)}
						<span className="text-sm">{field.label}</span>
					</label>
				);
			case "select": {
				const options = field.options ?? [];
				const raw = value == null ? "" : String(value);
				return (
					<Select
						value={raw === "" ? EMPTY_VALUE : raw}
						onValueChange={(next) => onChange(next === EMPTY_VALUE ? "" : next)}
					>
						<SelectTrigger id={fieldId} className="bg-transparent">
							<SelectValue placeholder={field.placeholder ?? "请选择"} />
						</SelectTrigger>
						<SelectContent>
							{options.map((o) => (
								<SelectItem
									key={o.value || EMPTY_VALUE}
									value={o.value || EMPTY_VALUE}
								>
									{o.label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				);
			}
			case "textarea":
				return (
					<textarea
						id={fieldId}
						name={field.name}
						className={clsx(inputClass, "h-auto py-2", field.className)}
						placeholder={field.placeholder}
						rows={3}
						value={value ?? ""}
						autoComplete="off"
						onChange={(e) => onChange(e.currentTarget.value)}
						onBlur={onBlur}
					/>
				);
			case "number":
				return (
					<input
						id={fieldId}
						type="number"
						name={field.name}
						className={inputClass}
						placeholder={field.placeholder}
						value={value ?? ""}
						autoComplete="off"
						onChange={(e) =>
							onChange(
								e.currentTarget.value === ""
									? ""
									: Number(e.currentTarget.value),
							)
						}
						onBlur={onBlur}
					/>
				);
			default:
				return (
					<input
						id={fieldId}
						type={field.type}
						name={field.name}
						className={inputClass}
						placeholder={field.placeholder}
						value={value ?? ""}
						// 房间名 + 密码同表单时，浏览器会把它当登录表单自动填用户名/密码；
						// 密码用 new-password 明确告知这是新密码，其余字段关闭填充。
						autoComplete={field.type === "password" ? "new-password" : "off"}
						onChange={(e) => onChange(e.currentTarget.value)}
						onBlur={onBlur}
					/>
				);
		}
	})();

	return (
		<div
			className={clsx(
				"relative mb-2",
				field.className && field.type !== "textarea" && field.className,
			)}
		>
			{field.type !== "switch" && field.type !== "checkbox" && (
				<label htmlFor={fieldId} className="mb-1 block text-sm">
					{field.label}
					{field.required && <span className="text-destructive ml-0.5">*</span>}
				</label>
			)}
			{body}
			{showError && (
				<p className="text-destructive absolute top-full left-1 mt-0.5 text-xs">
					{error}
				</p>
			)}
		</div>
	);
}

export function Form({
	fields,
	defaultValues = {},
	formClassName,
	showSubmitButton = true,
	submitButtonText = "提交",
	onSubmit,
}: FormProps) {
	const [values, setValues] = useState<Record<string, any>>(() => ({
		...Object.fromEntries(fields.map((f) => [f.name, undefined])),
		...defaultValues,
	}));
	const [touched, setTouched] = useState<Record<string, boolean>>({});
	const [submitting, setSubmitting] = useState(false);

	const handleChange = (name: string, v: any) => {
		setValues((prev) => ({ ...prev, [name]: v }));
	};

	const handleSubmit = async (e: FormEvent) => {
		e.preventDefault();
		setTouched(Object.fromEntries(fields.map((f) => [f.name, true])));
		const hasError = fields.some((f) => f.validation?.(values[f.name]));
		if (hasError) return;
		setSubmitting(true);
		try {
			await onSubmit(values);
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<form
			noValidate
			autoComplete="off"
			onSubmit={handleSubmit}
			className={formClassName}
		>
			{fields.map((field) => (
				<Field
					key={field.name}
					field={field}
					value={values[field.name]}
					touched={!!touched[field.name]}
					error={field.validation?.(values[field.name])}
					onChange={(v) => handleChange(field.name, v)}
					onBlur={() => setTouched((prev) => ({ ...prev, [field.name]: true }))}
				/>
			))}
			{showSubmitButton && (
				<div className="mt-3 flex justify-end">
					<Button type="submit" disabled={submitting}>
						{submitting ? "提交中..." : submitButtonText}
					</Button>
				</div>
			)}
		</form>
	);
}

export type { FormFieldConfig as FormFieldConfigType };
