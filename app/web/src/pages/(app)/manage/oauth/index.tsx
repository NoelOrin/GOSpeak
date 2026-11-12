import { Button, toast } from "@gospeak/components";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { LogIn, Plus, RefreshCcw, X } from "lucide-react";
import { useState } from "react";
import {
	createProvider,
	deleteProvider,
	listProviders,
	type OAuthProvider,
	updateProvider,
} from "@/api/oauth";
import { ManageHeader, ManagePage } from "@/components/manage/ManageShell";
import ConfirmModal from "@/components/common/ConfirmModal";
import { denyAccess } from "@/utils/accessDenied";
import { hasPermission } from "@/utils/permissions";
import { toastActionError } from "@/utils/toast";
import OAuthProviderForm from "./components/OAuthProviderForm";
import OAuthProviderTable from "./components/OAuthProviderTable";
import ProviderPresetPicker from "./components/ProviderPresetPicker";
import { NATIVE_PROVIDERS, type ProviderPreset } from "./components/presets";

export const Route = createFileRoute("/(app)/manage/oauth/")({
	beforeLoad: () => {
		if (!hasPermission("oauth:read")) {
			denyAccess();
		}
	},
	component: OAuthPage,
	staticData: {
		title: "OAuth",
	},
});

function OAuthPage() {
	const _queryClient = useQueryClient();
	const {
		data: providersData,
		isLoading,
		refetch,
	} = useQuery({
		queryKey: ["oauth-providers-manage"],
		queryFn: listProviders,
	});
	const [formMode, setFormMode] = useState<"hidden" | "preset" | "form">(
		"hidden",
	);
	const [editingId, setEditingId] = useState<number | null>(null);
	const [saving, setSaving] = useState(false);
	const [deletingId, setDeletingId] = useState<number | null>(null);
	const [pendingDelete, setPendingDelete] = useState<OAuthProvider | null>(
		null,
	);

	// 表单字段
	const [name, setName] = useState("");
	const [displayName, setDisplayName] = useState("");
	const [iconURL, setIconURL] = useState("");
	const [clientID, setClientID] = useState("");
	const [clientSecret, setClientSecret] = useState("");
	const [authURL, setAuthURL] = useState("");
	const [tokenURL, setTokenURL] = useState("");
	const [userInfoURL, setUserInfoURL] = useState("");
	const [redirectURL, setRedirectURL] = useState("");
	const [scopes, setScopes] = useState("");
	const [uidField, setUidField] = useState("");
	const [usernameField, setUsernameField] = useState("");
	const [avatarField, setAvatarField] = useState("");
	const [emailField, setEmailField] = useState("");
	const [enabled, setEnabled] = useState(true);

	const isNative = (providerName: string) => NATIVE_PROVIDERS.has(providerName);
	const isEditing = editingId !== null;
	const showFieldMapping = !isNative(name);

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
		if (saving) return;
		if (!name.trim()) return;
		setSaving(true);
		try {
			if (isEditing) {
				const input: any = {
					id: editingId ?? 0,
					name: name.trim(),
					display_name: displayName.trim(),
					icon_url: iconURL.trim(),
					client_id: clientID.trim(),
					auth_url: authURL.trim(),
					token_url: tokenURL.trim(),
					user_info_url: userInfoURL.trim(),
					redirect_url: redirectURL.trim(),
					scopes: scopes.trim(),
					uid_field: uidField.trim(),
					username_field: usernameField.trim(),
					avatar_field: avatarField.trim(),
					email_field: emailField.trim(),
					enabled,
				};
				if (clientSecret) input.client_secret = clientSecret.trim();
				await updateProvider(input);
			} else {
				const input: any = {
					name: name.trim(),
					display_name: displayName.trim(),
					icon_url: iconURL.trim(),
					client_id: clientID.trim(),
					client_secret: clientSecret.trim(),
					auth_url: authURL.trim(),
					token_url: tokenURL.trim(),
					user_info_url: userInfoURL.trim(),
					redirect_url: redirectURL.trim(),
					scopes: scopes.trim(),
					uid_field: uidField.trim(),
					username_field: usernameField.trim(),
					avatar_field: avatarField.trim(),
					email_field: emailField.trim(),
					enabled,
				};
				await createProvider(input);
			}
			closeForm();
			void refetch();
			toast.success(isEditing ? "提供商已更新" : "提供商已创建");
		} catch (error) {
			toastActionError(error, "保存 OAuth 提供商失败");
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (p: OAuthProvider) => {
		setPendingDelete(p);
	};

	const confirmDelete = async () => {
		const p = pendingDelete;
		if (!p) return;
		setDeletingId(p.id);
		try {
			await deleteProvider(p.id);
			void refetch();
			toast.success("提供商已删除");
			setPendingDelete(null);
		} catch (error) {
			toastActionError(error, "删除 OAuth 提供商失败");
		} finally {
			setDeletingId(null);
		}
	};

	const existingNames = new Set((providersData ?? []).map((p) => p.name));

	return (
		<ManagePage>
			<ManageHeader
				icon={<LogIn size={20} />}
				title="OAuth 登录配置"
				description="管理第三方登录提供商"
				actions={
					<>
						<button
							type="button"
							className="hover:bg-muted flex size-9 cursor-pointer items-center justify-center rounded-sm transition-colors"
							onClick={() => void refetch()}
							title="刷新"
						>
							<RefreshCcw size={16} />
						</button>
						{formMode === "hidden" && (
							<Button
								type="button"
								variant="outline"
								className="gap-1.5"
								onClick={openCreateForm}
							>
								<Plus size={16} /> 添加
							</Button>
						)}
						{formMode !== "hidden" && (
							<Button
								type="button"
								variant="ghost"
								className="gap-1.5"
								onClick={closeForm}
							>
								<X size={16} /> 关闭
							</Button>
						)}
					</>
				}
			/>

			{/* 预设选择 */}
			{formMode === "preset" && (
				<ProviderPresetPicker
					existingNames={existingNames}
					onBack={closeForm}
					onSelect={applyPreset}
					onCustom={openCustomForm}
				/>
			)}

			{/* 表单 */}
			{formMode === "form" && (
				<OAuthProviderForm
					isEditing={isEditing}
					isNative={isNative(name)}
					showFieldMapping={showFieldMapping}
					saving={saving}
					name={name}
					displayName={displayName}
					iconURL={iconURL}
					clientID={clientID}
					clientSecret={clientSecret}
					authURL={authURL}
					tokenURL={tokenURL}
					userInfoURL={userInfoURL}
					redirectURL={redirectURL}
					scopes={scopes}
					uidField={uidField}
					usernameField={usernameField}
					avatarField={avatarField}
					emailField={emailField}
					enabled={enabled}
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
			)}

			{/* 提供商列表 */}
			<OAuthProviderTable
				loading={isLoading}
				providers={providersData ?? []}
				deletingId={deletingId}
				onEdit={openEditForm}
				onDelete={(p) => void handleDelete(p)}
			/>

			<ConfirmModal
				open={!!pendingDelete}
				title="删除 OAuth 提供商"
				message={
					<span>
						确认删除「
						{pendingDelete?.display_name || pendingDelete?.name}
						」？删除后该第三方登录入口立即失效，已绑定的用户将无法再用它登录。
					</span>
				}
				confirmText="删除"
				loading={!!deletingId}
				onClose={() => setPendingDelete(null)}
				onConfirm={confirmDelete}
			/>
		</ManagePage>
	);
}
