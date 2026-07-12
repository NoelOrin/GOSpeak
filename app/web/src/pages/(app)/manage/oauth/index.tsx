import { createFileRoute, redirect } from "@tanstack/solid-router";
import userStore from "@/stores/userStore";
import { createResource, createSignal, For, Show } from "solid-js";
import { showToast } from "solid-notifications";
import LogIn from "lucide-solid/icons/log-in";
import RefreshCcw from "lucide-solid/icons/refresh-ccw";
import Plus from "lucide-solid/icons/plus";
import Pencil from "lucide-solid/icons/pencil";
import Trash2 from "lucide-solid/icons/trash-2";
import X from "lucide-solid/icons/x";
import Save from "lucide-solid/icons/save";
import ArrowLeft from "lucide-solid/icons/arrow-left";
import {
  listProviders,
  createProvider,
  updateProvider,
  deleteProvider,
  type OAuthProvider,
} from "@/api/oauth";

export const Route = createFileRoute("/(app)/manage/oauth/")({
  beforeLoad: () => {
    if (userStore.user()?.role !== "admin") {
      throw redirect({ to: "/" });
    }
  },
  component: OAuthPage,
  staticData: {
    title: "OAuth",
    icon: "icon-manage",
  },
});

// 后端有专用实现的提供商（GitHubProvider / GoogleProvider / QQProvider）
const NATIVE_PROVIDERS = new Set(["github", "google", "qq"]);

// 常用 OAuth 提供商预设模板
interface ProviderPreset {
  name: string;
  label: string;
  auth_url: string;
  token_url: string;
  user_info_url: string;
  scopes: string;
  uid_field: string;
  username_field: string;
  avatar_field: string;
  email_field: string;
}

const PROVIDER_PRESETS: ProviderPreset[] = [
  {
    name: "github",
    label: "GitHub",
    auth_url: "https://github.com/login/oauth/authorize",
    token_url: "https://github.com/login/oauth/access_token",
    user_info_url: "https://api.github.com/user",
    scopes: "read:user user:email",
    uid_field: "id",
    username_field: "login",
    avatar_field: "avatar_url",
    email_field: "email",
  },
  {
    name: "google",
    label: "Google",
    auth_url: "https://accounts.google.com/o/oauth2/v2/auth",
    token_url: "https://oauth2.googleapis.com/token",
    user_info_url: "https://www.googleapis.com/oauth2/v3/userinfo",
    scopes: "openid email profile",
    uid_field: "sub",
    username_field: "name",
    avatar_field: "picture",
    email_field: "email",
  },
  {
    name: "qq",
    label: "QQ",
    auth_url: "https://graph.qq.com/oauth2.0/authorize",
    token_url: "https://graph.qq.com/oauth2.0/token",
    user_info_url: "https://graph.qq.com/user/get_user_info",
    scopes: "get_user_info",
    uid_field: "openid",
    username_field: "nickname",
    avatar_field: "figureurl_qq_2",
    email_field: "",
  },
  {
    name: "gitlab",
    label: "GitLab",
    auth_url: "https://gitlab.com/oauth/authorize",
    token_url: "https://gitlab.com/oauth/token",
    user_info_url: "https://gitlab.com/api/v4/user",
    scopes: "read_user",
    uid_field: "id",
    username_field: "username",
    avatar_field: "avatar_url",
    email_field: "email",
  },
  {
    name: "discord",
    label: "Discord",
    auth_url: "https://discord.com/api/oauth2/authorize",
    token_url: "https://discord.com/api/oauth2/token",
    user_info_url: "https://discord.com/api/users/@me",
    scopes: "identify email",
    uid_field: "id",
    username_field: "username",
    avatar_field: "avatar",
    email_field: "email",
  },
  {
    name: "microsoft",
    label: "Microsoft",
    auth_url: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
    token_url: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
    user_info_url: "https://graph.microsoft.com/oidc/userinfo",
    scopes: "openid email profile",
    uid_field: "sub",
    username_field: "name",
    avatar_field: "picture",
    email_field: "email",
  },
];

function OAuthPage() {
  const [providersData, { refetch }] = createResource(() => listProviders());
  const [formMode, setFormMode] = createSignal<"hidden" | "preset" | "form">("hidden");
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
    if (!name().trim()) {
      showToast("请填写提供商名称", { type: "warning" });
      return;
    }
    setSaving(true);
    try {
      if (isEditing()) {
        const input: any = {
          id: editingId()!,
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
        const res = await updateProvider(input);
        if (res.code !== 0) {
          showToast(res.msg || "更新失败", { type: "error" });
          return;
        }
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
        const res = await createProvider(input);
        if (res.code !== 0) {
          showToast(res.msg || "创建失败", { type: "error" });
          return;
        }
        showToast("已创建", { type: "success" });
      }
      closeForm();
      refetch();
    } catch (e: any) {
      showToast(e?.message || "操作失败", { type: "error" });
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (p: OAuthProvider) => {
    if (!confirm(`确认删除 OAuth 提供商「${p.display_name || p.name}」？`)) return;
    setDeletingId(p.id);
    try {
      const res = await deleteProvider(p.id);
      if (res.code !== 0) {
        showToast(res.msg || "删除失败", { type: "error" });
        return;
      }
      showToast("已删除", { type: "success" });
      refetch();
    } catch (e: any) {
      showToast(e?.message || "删除失败", { type: "error" });
    } finally {
      setDeletingId(null);
    }
  };

  const existingNames = () => new Set((providersData()?.data ?? []).map((p) => p.name));

  return (
    <div class="flex h-full min-h-0 flex-col gap-4 p-4 overflow-auto">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <LogIn size={20} />
          <h3 class="font-bold text-lg">OAuth 登录配置</h3>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-ghost btn-sm" onClick={() => void refetch()} title="刷新">
            <RefreshCcw size={16} />
          </button>
          <Show when={formMode() === "hidden"}>
            <button class="btn btn-primary btn-sm" onClick={openCreateForm}>
              <Plus size={16} /> 添加
            </button>
          </Show>
          <Show when={formMode() !== "hidden"}>
            <button class="btn btn-ghost btn-sm" onClick={closeForm}>
              <X size={16} /> 关闭
            </button>
          </Show>
        </div>
      </div>

      {/* 预设选择 */}
      <Show when={formMode() === "preset"}>
        <div class="card bg-base-200 shadow-sm">
          <div class="card-body gap-4">
            <div class="flex items-center gap-2">
              <button class="btn btn-ghost btn-xs" onClick={() => setFormMode("hidden")}>
                <ArrowLeft size={14} />
              </button>
              <h3 class="font-bold text-base">选择 OAuth 提供商</h3>
            </div>
            <p class="text-xs text-base-content/50">
              选择常用提供商可自动填充端点和字段映射，也可选择自定义手动配置
            </p>

            <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
              <For each={PROVIDER_PRESETS}>
                {(preset) => {
                  const alreadyExists = () => existingNames().has(preset.name);
                  return (
                    <button
                      type="button"
                      class="flex flex-col items-center gap-2 rounded-lg border border-base-300 p-4 transition hover:border-primary hover:bg-primary/5"
                      classList={{ "opacity-50 cursor-not-allowed": alreadyExists() }}
                      disabled={alreadyExists()}
                      onClick={() => applyPreset(preset)}
                    >
                      <ProviderIcon name={preset.name} size={32} />
                      <span class="text-sm font-medium">{preset.label}</span>
                      <Show when={alreadyExists()}>
                        <span class="badge badge-ghost badge-xs">已配置</span>
                      </Show>
                    </button>
                  );
                }}
              </For>

              {/* 自定义 */}
              <button
                type="button"
                class="flex flex-col items-center gap-2 rounded-lg border border-dashed border-base-300 p-4 transition hover:border-primary hover:bg-primary/5"
                onClick={openCustomForm}
              >
                <div class="flex h-8 w-8 items-center justify-center rounded-full bg-base-300">
                  <Plus size={16} />
                </div>
                <span class="text-sm font-medium">自定义</span>
                <span class="text-xs text-base-content/50">手动填写端点</span>
              </button>
            </div>
          </div>
        </div>
      </Show>

      {/* 表单 */}
      <Show when={formMode() === "form"}>
        <div class="card bg-base-200 shadow-sm">
          <div class="card-body gap-3">
            <div class="flex items-center gap-2">
              <Show when={!isEditing()}>
                <button class="btn btn-ghost btn-xs" onClick={() => setFormMode("preset")}>
                  <ArrowLeft size={14} />
                </button>
              </Show>
              <h3 class="font-bold text-base">
                {isEditing() ? "编辑提供商" : `配置 ${displayName() || name()}`}
              </h3>
              <Show when={!isNative(name()) && !isEditing()}>
                <span class="badge badge-info badge-sm">通用 OAuth2</span>
              </Show>
              <Show when={isNative(name())}>
                <span class="badge badge-ghost badge-sm">内置实现</span>
              </Show>
            </div>

            <div class="divider my-0 text-xs text-base-content/40">基本信息</div>
            <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
              <Field label="名称（唯一标识）">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="如 my-oauth、gitlab、gitea"
                  value={name()}
                  onInput={(e) => setName(e.currentTarget.value)}
                  disabled={isEditing()}
                />
              </Field>
              <Field label="显示名称">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="登录页显示的名称"
                  value={displayName()}
                  onInput={(e) => setDisplayName(e.currentTarget.value)}
                />
              </Field>
              <Field label="图标 URL">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="https://example.com/icon.svg"
                  value={iconURL()}
                  onInput={(e) => setIconURL(e.currentTarget.value)}
                />
              </Field>
            </div>

            <div class="divider my-0 text-xs text-base-content/40">OAuth 端点</div>
            <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
              <Field label="Client ID">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="OAuth App Client ID"
                  value={clientID()}
                  onInput={(e) => setClientID(e.currentTarget.value)}
                />
              </Field>
              <Field label="Client Secret">
                <input
                  type="password"
                  class="input input-bordered input-sm w-full"
                  placeholder={isEditing() ? "留空保持原值" : "OAuth App Client Secret"}
                  value={clientSecret()}
                  onInput={(e) => setClientSecret(e.currentTarget.value)}
                />
              </Field>
              <Field label="授权 URL (Auth URL)">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="https://example.com/oauth/authorize"
                  value={authURL()}
                  onInput={(e) => setAuthURL(e.currentTarget.value)}
                />
              </Field>
              <Field label="Token URL">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="https://example.com/oauth/token"
                  value={tokenURL()}
                  onInput={(e) => setTokenURL(e.currentTarget.value)}
                />
              </Field>
              <Field label="UserInfo URL">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="https://example.com/api/user"
                  value={userInfoURL()}
                  onInput={(e) => setUserInfoURL(e.currentTarget.value)}
                />
              </Field>
              <Field label="回调 URL (Redirect URL)">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="https://your-domain/api/v1/oauth/callback/my-oauth"
                  value={redirectURL()}
                  onInput={(e) => setRedirectURL(e.currentTarget.value)}
                />
              </Field>
              <Field label="Scopes">
                <input
                  type="text"
                  class="input input-bordered input-sm w-full"
                  placeholder="openid profile email"
                  value={scopes()}
                  onInput={(e) => setScopes(e.currentTarget.value)}
                />
              </Field>
            </div>

            <Show when={showFieldMapping()}>
              <div class="divider my-0 text-xs text-base-content/40">
                用户信息字段映射
              </div>
              <p class="text-xs text-base-content/50">
                指定 UserInfo API 返回 JSON 中各字段的 key 名，用于解析用户信息
              </p>
              <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
                <Field label="UID 字段名">
                  <input
                    type="text"
                    class="input input-bordered input-sm w-full"
                    placeholder="id（默认）"
                    value={uidField()}
                    onInput={(e) => setUidField(e.currentTarget.value)}
                  />
                </Field>
                <Field label="用户名字段名">
                  <input
                    type="text"
                    class="input input-bordered input-sm w-full"
                    placeholder="username（默认）"
                    value={usernameField()}
                    onInput={(e) => setUsernameField(e.currentTarget.value)}
                  />
                </Field>
                <Field label="头像字段名">
                  <input
                    type="text"
                    class="input input-bordered input-sm w-full"
                    placeholder="avatar_url（默认）"
                    value={avatarField()}
                    onInput={(e) => setAvatarField(e.currentTarget.value)}
                  />
                </Field>
                <Field label="邮箱字段名">
                  <input
                    type="text"
                    class="input input-bordered input-sm w-full"
                    placeholder="email（默认）"
                    value={emailField()}
                    onInput={(e) => setEmailField(e.currentTarget.value)}
                  />
                </Field>
              </div>
            </Show>

            <div class="flex items-center justify-between py-2">
              <span class="text-sm font-medium">启用</span>
              <input
                type="checkbox"
                class="toggle toggle-sm toggle-primary"
                checked={enabled()}
                onChange={(e) => setEnabled(e.currentTarget.checked)}
              />
            </div>

            <div class="flex gap-2">
              <button
                type="button"
                class="btn btn-primary btn-sm"
                classList={{ "btn-disabled": saving() }}
                onClick={handleSave}
              >
                <Show when={saving()} fallback={<Save size={16} />}>
                  <span class="loading loading-spinner loading-xs" />
                </Show>
                {saving() ? "保存中..." : "保存"}
              </button>
              <button type="button" class="btn btn-ghost btn-sm" onClick={closeForm}>
                取消
              </button>
            </div>
          </div>
        </div>
      </Show>

      {/* 提供商列表 */}
      <div>
        <Show
          when={!providersData.loading}
          fallback={<div class="loading loading-spinner loading-sm" />}
        >
          <Show
            when={(providersData()?.data?.length ?? 0) > 0}
            fallback={
              <div class="text-base-content/50 py-8 text-center text-sm">
                暂无 OAuth 提供商配置，点击「添加」开始配置
              </div>
            }
          >
            <div class="overflow-x-auto">
              <table class="table table-zebra table-sm">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>显示名称</th>
                    <th>类型</th>
                    <th>回调 URL</th>
                    <th>状态</th>
                    <th>操作</th>
                  </tr>
                </thead>
                <tbody>
                  <For each={providersData()?.data ?? []}>
                    {(p) => (
                      <tr>
                        <td class="font-medium">
                          <div class="flex items-center gap-2">
                            <ProviderIcon name={p.name} size={16} />
                            <span class="font-mono">{p.name}</span>
                          </div>
                        </td>
                        <td>{p.display_name || "-"}</td>
                        <td>
                          <Show
                            when={isNative(p.name)}
                            fallback={
                              <span class="badge badge-info badge-sm">通用</span>
                            }
                          >
                            <span class="badge badge-ghost badge-sm">内置</span>
                          </Show>
                        </td>
                        <td class="max-w-48 truncate text-xs text-base-content/60">
                          {p.redirect_url || "-"}
                        </td>
                        <td>
                          <Show
                            when={p.enabled}
                            fallback={
                              <span class="badge badge-ghost badge-sm">已禁用</span>
                            }
                          >
                            <span class="badge badge-success badge-sm">启用</span>
                          </Show>
                        </td>
                        <td>
                          <div class="flex gap-1">
                            <button
                              type="button"
                              class="btn btn-ghost btn-xs"
                              onClick={() => openEditForm(p)}
                            >
                              <Pencil size={14} />
                            </button>
                            <button
                              type="button"
                              class="btn btn-ghost btn-xs text-error"
                              disabled={deletingId() === p.id}
                              onClick={() => handleDelete(p)}
                            >
                              <Show
                                when={deletingId() === p.id}
                                fallback={<Trash2 size={14} />}
                              >
                                <span class="loading loading-spinner loading-xs" />
                              </Show>
                            </button>
                          </div>
                        </td>
                      </tr>
                    )}
                  </For>
                </tbody>
              </table>
            </div>
          </Show>
        </Show>
      </div>
    </div>
  );
}

interface FieldProps {
  label: string;
  children: any;
}

const Field = (props: FieldProps) => (
  <fieldset class="fieldset">
    <legend class="fieldset-legend text-[14px]">{props.label}</legend>
    {props.children}
  </fieldset>
);

interface ProviderIconProps {
  name: string;
  size?: number;
}

const ProviderIcon = (props: ProviderIconProps) => {
  const s = () => props.size ?? 24;
  switch (props.name) {
    case "github":
      return (
        <svg viewBox="0 0 24 24" width={s()} height={s()} fill="currentColor" class="text-base-content">
          <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
        </svg>
      );
    case "google":
      return (
        <svg viewBox="0 0 24 24" width={s()} height={s()}>
          <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" />
          <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" />
          <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" />
          <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" />
        </svg>
      );
    case "qq":
      return (
        <svg viewBox="0 0 24 24" width={s()} height={s()} fill="#1EBAFC">
          <path d="M21.395 15.035a40 40 0 0 0-.803-2.264l-1.079-2.695c.001-.032.014-.562.014-.836C19.526 4.632 17.351 0 12 0S4.474 4.632 4.474 9.241c0 .274.013.804.014.836l-1.08 2.695a39 39 0 0 0-.802 2.264c-1.021 3.283-.69 4.643-.438 4.673.54.065 2.103-2.472 2.103-2.472 0 1.469.756 3.387 2.394 4.771-.612.188-1.363.479-1.845.835-.434.32-.379.646-.301.778.343.578 5.883.369 7.482.189 1.6.18 7.14.389 7.483-.189.078-.132.132-.458-.301-.778-.483-.356-1.233-.646-1.846-.836 1.637-1.384 2.393-3.302 2.393-4.771 0 0 1.563 2.537 2.103 2.472.251-.03.581-1.39-.438-4.673"/>
        </svg>
      );
    case "gitlab":
      return (
        <svg viewBox="0 0 24 24" width={s()} height={s()} fill="currentColor" class="text-[#FC6D26]">
          <path d="M23.955 13.587l-1.347-4.135-2.668-8.213a.455.455 0 00-.867 0L16.406 9.45H7.594L4.927 1.239a.455.455 0 00-.867 0L1.392 9.452.045 13.587a.924.924 0 00.331 1.022L12 23.054l11.624-8.445a.92.92 0 00.331-1.022" />
        </svg>
      );
    case "discord":
      return (
        <svg viewBox="0 0 24 24" width={s()} height={s()} fill="#5865F2">
          <path d="M20.317 4.37a19.79 19.79 0 00-4.885-1.515.074.074 0 00-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 00-5.487 0 12.64 12.64 0 00-.617-1.25.077.077 0 00-.079-.037A19.736 19.736 0 003.677 4.37a.07.07 0 00-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 00.031.057 19.9 19.9 0 005.993 3.03.078.078 0 00.084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 00-.041-.106 13.107 13.107 0 01-1.872-.892.077.077 0 01-.008-.128 10.2 10.2 0 00.372-.292.074.074 0 01.077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 01.078.01c.12.098.246.198.373.292a.077.077 0 01-.006.127 12.299 12.299 0 01-1.873.892.077.077 0 00-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 00.084.028 19.839 19.839 0 006.002-3.03.077.077 0 00.032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 00-.031-.03zM8.02 15.331c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z" />
        </svg>
      );
    case "microsoft":
      return (
        <svg viewBox="0 0 24 24" width={s()} height={s()}>
          <path fill="#F25022" d="M1 1h10v10H1z" />
          <path fill="#7FBA00" d="M13 1h10v10H13z" />
          <path fill="#00A4EF" d="M1 13h10v10H1z" />
          <path fill="#FFB900" d="M13 13h10v10H13z" />
        </svg>
      );
    default:
      return (
        <div
          class="flex items-center justify-center rounded-full bg-base-300 text-base-content"
          style={{ width: `${s()}px`, height: `${s()}px` }}
        >
          <LogIn size={Math.floor(s() * 0.5)} />
        </div>
      );
  }
};
