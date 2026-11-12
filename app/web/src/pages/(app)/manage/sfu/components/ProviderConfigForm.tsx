import type { SFUProvider } from "@gospeak/sfu-client/types";
import type { UpdateSFUConfigParams } from "@/api/sfu";
import PasswordInput from "@/components/common/passwordInput";
import type { SecretFlags } from "./constants";
import type { FieldErrors } from "./validation";

export interface ProviderConfigFormProps {
	provider: SFUProvider;
	form: UpdateSFUConfigParams;
	errors: FieldErrors;
	secretFlags: SecretFlags;
	saving: boolean;
	updateField: <K extends keyof UpdateSFUConfigParams>(
		key: K,
		value: UpdateSFUConfigParams[K],
	) => void;
}

const inputClass =
	"border-input bg-card text-foreground focus-visible:ring-ring placeholder:text-faint-foreground flex h-9 w-full rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50";
const inputErrorClass =
	"border-input bg-card text-destructive focus-visible:ring-destructive placeholder:text-faint-foreground flex h-9 w-full rounded-sm border px-3 text-sm focus-visible:outline-none focus-visible:ring-2 disabled:cursor-not-allowed disabled:opacity-50";

function FormField({
	label,
	error,
	children,
}: {
	label: string;
	error?: string;
	children: React.ReactNode;
}) {
	return (
		<label className="block">
			<span className="mb-1 block text-sm">{label}</span>
			{children}
			{error && <p className="text-destructive mt-1 text-xs">{error}</p>}
		</label>
	);
}

export default function ProviderConfigForm({
	provider,
	form,
	errors,
	secretFlags,
	saving,
	updateField,
}: ProviderConfigFormProps) {
	const cls = (err?: string) => (err ? inputErrorClass : inputClass);

	if (provider === "livekit") {
		return (
			<div className="grid grid-cols-1 gap-x-4 gap-y-3 md:grid-cols-2">
				<FormField label="Host" error={errors.livekit_host}>
					<input
						type="text"
						className={cls(errors.livekit_host)}
						placeholder="wss://livekit.example.com"
						value={form.livekit_host ?? ""}
						onChange={(event) =>
							updateField("livekit_host", event.currentTarget.value)
						}
						disabled={saving}
					/>
				</FormField>
				<FormField label="API Key" error={errors.livekit_key}>
					<input
						type="text"
						className={cls(errors.livekit_key)}
						placeholder="API key"
						value={form.livekit_key ?? ""}
						onChange={(event) =>
							updateField("livekit_key", event.currentTarget.value)
						}
						disabled={saving}
					/>
				</FormField>
				<FormField label="API Secret" error={errors.livekit_secret}>
					<PasswordInput
						native
						autoComplete="off"
						className={cls(errors.livekit_secret)}
						placeholder={
							secretFlags.livekit_secret_set ? "已配置，留空保留" : "API secret"
						}
						value={form.livekit_secret ?? ""}
						onChange={(event) =>
							updateField("livekit_secret", event.currentTarget.value)
						}
						disabled={saving}
					/>
				</FormField>
			</div>
		);
	}

	return null;
}
