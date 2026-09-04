import { createFileRoute, redirect } from "@tanstack/solid-router";
import LogIn from "lucide-solid/icons/log-in";
import Plus from "lucide-solid/icons/plus";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import X from "lucide-solid/icons/x";
import { createResource, createSignal, Show } from "solid-js";
import { showToast } from "solid-notifications";
import {
	createProvider,
	deleteProvider,
	listProviders,
	type OAuthProvider,
	updateProvider,
} from "@/api/oauth";
import { ManageHeader, ManagePage } from "@/components/manage/ManageShell";
import { hasPermission } from "@/utils/permissions";
import OAuthProviderForm from "./components/OAuthProviderForm";
import OAuthProviderTable from "./components/OAuthProviderTable";
import ProviderPresetPicker from "./components/ProviderPresetPicker";
import { NATIVE_PROVIDERS, type ProviderPreset } from "./components/presets";

export const Route = createFileRoute("/(app)/manage/oauth/")({
	beforeLoad: () => {
		if (!hasPermission("oauth:read")) {
			throw redirect({ to: "/" });
		}
	},
	component: OAuthPage,
	staticData: {
		title: "OAuth",
		icon: "icon-manage",
	},
});

function OAuthPage() {
	const [providersData, { refetch }] = createResource(() => listProviders());
	const [formMode, setFormMode] = createSignal<"hidden" | "preset" | "form">(
		"hidden",
	);
	const [editingId, setEditingId] = createSignal<number | null>(null);
	const [saving, setSaving] = createSignal(false);
	const [deletingId, setDeletingId] = createSignal<number | null>(null);

	// 表单字段
	const [name, setName] = createSignal("");
	const [displayName, setDisplayName] = createSignal("");
	const [iconURL, setIconURL] = createSignal("");
	const [clientID, setClientID] = createSignal("");
	const [clientSecret, setClientSecret] = createSignal("");
	const [authURL, setAuthURL] = createSignal("");
	const [tokenURL, setTokenURL] = createSignal("");
	const [userInfoURL, setUserInfoURL] = createSignal("");
	const [redirectURL, setRedirectURL] = createSignal("");
	const [scopes, setScopes] = createSignal("");
	const [uidField, setUidField] = createSignal("");
	const [usernameField, setUsernameField] = createSignal("");
	const [avatarField, setAvatarField] = createSignal("");
	const [emailField, setEmailField] = createSignal("");
	const [enabled, setEnabled] = createSignal(true);

	const isNative = (providerName: string) => NATIVE_PROVIDERS.has(providerName);
	const isEditing = () => editingId() !== null;
	const showFieldMapping = () => !isNative(name());

	const resetForm = () => {
		setEditingId(null);
		setName("");
		setDisplayName("");
		setIconURL("");
		setClientID("");
		setClientSecret("");
		setAuthURL("");
		setTokenURL("");
		setUserInfoURL("");
		setRedirectURL("");
		setScopes("");
		setUidField("");
		setUsernameField("");
		setAvatarField("");
		setEmailField("");
		setEnabled(true);
	};

	const applyPreset = (preset: ProviderPreset) => {
		resetForm();
		setName(preset.name);
		setDisplayName(preset.label);

		setAuthURL(preset.auth_url);
		setTokenURL(preset.token_url);
		setUserInfoURL(preset.user_info_url);
		setScopes(preset.scopes);
		setUidField(preset.uid_field);
		setUsernameField(preset.username_field);
		setAvatarField(preset.avatar_field);
		setEmailField(preset.email_field);
		setFormMode("form");
	};

	const openCustomForm = () => {
		resetForm();
		setFormMode("form");
	};

	const openCreateForm = () => {
		setFormMode("preset");
	};

	const openEditForm = (p: OAuthProvider) => {
		setEditingId(p.id);
		setName(p.name);
		setDisplayName(p.display_name || "");
		setIconURL(p.icon_url || "");
		setClientID(p.client_id || "");
		setClientSecret("");
		setAuthURL(p.auth_url || "");
		setTokenURL(p.token_url || "");
		setUserInfoURL(p.userinfo_url || "");
		setRedirectURL(p.redirect_url || "");
		setScopes(p.scopes || "");
		setUidField(p.uid_field || "");
		setUsernameField(p.username_field || "");
		setAvatarField(p.avatar_field || "");
		setEmailField(p.email_field || "");
		setEnabled(p.enabled);
		setFormMode("form");
	};

	const closeForm = () => {
		setFormMode("hidden");
		resetForm();
	};

	const handleSave = async () => {
		if (saving()) return;
		if (!name().trim()) {
			showToast("请填写提供商名称", { type: "warning" });
			return;
		}
		setSaving(true);
		try {
			if (isEditing()) {
				const input: any = {
					id: editingId() ?? 0,
					name: name().trim(),
					display_name: displayName().trim(),
					icon_url: iconURL().trim(),
					client_id: clientID().trim(),
					auth_url: authURL().trim(),
					token_url: tokenURL().trim(),
					user_info_url: userInfoURL().trim(),
					redirect_url: redirectURL().trim(),
					scopes: scopes().trim(),
					uid_field: uidField().trim(),
					username_field: usernameField().trim(),
					avatar_field: avatarField().trim(),
					email_field: emailField().trim(),
					enabled: enabled(),
				};
				if (clientSecret()) input.client_secret = clientSecret().trim();
				await updateProvider(input);
				showToast("已更新", { type: "success" });
			} else {
				const input: any = {
					name: name().trim(),
					display_name: displayName().trim(),
					icon_url: iconURL().trim(),
					client_id: clientID().trim(),
					client_secret: clientSecret().trim(),
					auth_url: authURL().trim(),
					token_url: tokenURL().trim(),
					user_info_url: userInfoURL().trim(),
					redirect_url: redirectURL().trim(),
					scopes: scopes().trim(),
					uid_field: uidField().trim(),
					username_field: usernameField().trim(),
					avatar_field: avatarField().trim(),
					email_field: emailField().trim(),
					enabled: enabled(),
				};
				await createProvider(input);
				showToast("已创建", { type: "success" });
			}
			closeForm();
			refetch();
		} catch {
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (p: OAuthProvider) => {
		if (!confirm(`确认删除 OAuth 提供商「${p.display_name || p.name}」？`))
			return;
		setDeletingId(p.id);
		try {
			await deleteProvider(p.id);
			showToast("已删除", { type: "success" });
			refetch();
		} catch {
		} finally {
			setDeletingId(null);
		}
	};

	const existingNames = () =>
		new Set((providersData() ?? []).map((p) => p.name));

	return (
		<ManagePage>
			<ManageHeader
				icon={<LogIn size={18} />}
				title="OAuth 登录配置"
				description="管理第三方登录提供商"
				actions={
					<>
						<button
							class="btn btn-ghost btn-sm btn-square"
							onClick={() => void refetch()}
							title="刷新"
						>
							<RefreshCcw size={16} />
						</button>
						<Show when={formMode() === "hidden"}>
							<button
								class="btn btn-sm border border-base-300 bg-base-100 text-base-content shadow-none hover:bg-base-200"
								onClick={openCreateForm}
							>
								<Plus size={16} /> 添加
							</button>
						</Show>
						<Show when={formMode() !== "hidden"}>
							<button class="btn btn-ghost btn-sm" onClick={closeForm}>
								<X size={16} /> 关闭
							</button>
						</Show>
					</>
				}
			/>

			{/* 预设选择 */}
			<Show when={formMode() === "preset"}>
				<ProviderPresetPicker
					existingNames={existingNames()}
					onBack={closeForm}
					onSelect={applyPreset}
					onCustom={openCustomForm}
				/>
			</Show>

			{/* 表单 */}
			<Show when={formMode() === "form"}>
				<OAuthProviderForm
					isEditing={isEditing()}
					isNative={isNative(name())}
					showFieldMapping={showFieldMapping()}
					saving={saving()}
					name={name()}
					displayName={displayName()}
					iconURL={iconURL()}
					clientID={clientID()}
					clientSecret={clientSecret()}
					authURL={authURL()}
					tokenURL={tokenURL()}
					userInfoURL={userInfoURL()}
					redirectURL={redirectURL()}
					scopes={scopes()}
					uidField={uidField()}
					usernameField={usernameField()}
					avatarField={avatarField()}
					emailField={emailField()}
					enabled={enabled()}
					setName={setName}
					setDisplayName={setDisplayName}
					setIconURL={setIconURL}
					setClientID={setClientID}
					setClientSecret={setClientSecret}
					setAuthURL={setAuthURL}
					setTokenURL={setTokenURL}
					setUserInfoURL={setUserInfoURL}
					setRedirectURL={setRedirectURL}
					setScopes={setScopes}
					setUidField={setUidField}
					setUsernameField={setUsernameField}
					setAvatarField={setAvatarField}
					setEmailField={setEmailField}
					setEnabled={setEnabled}
					onBackToPreset={() => setFormMode("preset")}
					onSave={() => void handleSave()}
					onCancel={closeForm}
				/>
			</Show>

			{/* 提供商列表 */}
			<OAuthProviderTable
				loading={providersData.loading}
				providers={providersData() ?? []}
				deletingId={deletingId()}
				onEdit={openEditForm}
				onDelete={(p) => void handleDelete(p)}
			/>
		</ManagePage>
	);
}
