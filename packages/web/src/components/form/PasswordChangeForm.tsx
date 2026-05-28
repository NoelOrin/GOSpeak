import { createForm } from '@tanstack/solid-form'
import { createSignal, Show } from 'solid-js'

interface PasswordChangeFormProps {
  onSubmit: (values: { username?: string; oldPassword?: string; newPassword: string; name?: string }) => Promise<void>
  showUsername?: boolean
  showOldPassword?: boolean
  showName?: boolean
  submitText?: string
}

export default function PasswordChangeForm(props: PasswordChangeFormProps) {
  const [error, setError] = createSignal('')

  const form = createForm(() => ({
    defaultValues: {
      username: '',
      oldPassword: '',
      newPassword: '',
      confirmPassword: '',
      name: '',
    },
    onSubmit: async ({ value }) => {
      setError('')
      if (value.newPassword !== value.confirmPassword) {
        setError('两次输入的密码不一致')
        return
      }
      try {
        await props.onSubmit({
          username: props.showUsername ? value.username : undefined,
          oldPassword: props.showOldPassword ? value.oldPassword : undefined,
          newPassword: value.newPassword,
          name: props.showName ? value.name || undefined : undefined,
        })
      } catch (e: any) {
        setError(e?.response?.data?.msg || e?.message || '操作失败，请重试')
      }
    },
  }))

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        e.stopPropagation()
        form.handleSubmit()
      }}
    >
      <Show when={props.showUsername}>
        <form.Field
          name="username"
          validators={{
            onChange: ({ value }: { value: string }) => {
              if (!value) return '用户名 是必填项'
            },
          }}
        >
          {(field) => (
            <fieldset class="fieldset mb-3">
              <legend class="fieldset-legend text-[14px]">用户名</legend>
              <input
                type="text"
                value={field().state.value}
                placeholder="请输入用户名"
                class="input w-full"
                onInput={(e) => field().handleChange(e.currentTarget.value)}
              />
              <Show when={field().state.meta.isTouched && field().state.meta.errors.length > 0}>
                <p class="fieldset-label text-error">{field().state.meta.errors.join(', ')}</p>
              </Show>
            </fieldset>
          )}
        </form.Field>
      </Show>

      <Show when={props.showName}>
        <form.Field
          name="name"
        >
          {(field) => (
            <fieldset class="fieldset mb-3">
              <legend class="fieldset-legend text-[14px]">用户名（可选）</legend>
              <input
                type="text"
                value={field().state.value}
                placeholder="留空则保持不变"
                class="input w-full"
                onInput={(e) => field().handleChange(e.currentTarget.value)}
              />
            </fieldset>
          )}
        </form.Field>
      </Show>

      <Show when={props.showOldPassword}>
        <form.Field
          name="oldPassword"
          validators={{
            onChange: ({ value }: { value: string }) => {
              if (!value) return '旧密码 是必填项'
            },
          }}
        >
          {(field) => (
            <fieldset class="fieldset mb-3">
              <legend class="fieldset-legend text-[14px]">旧密码</legend>
              <input
                type="password"
                value={field().state.value}
                placeholder="请输入旧密码"
                class="input w-full"
                onInput={(e) => field().handleChange(e.currentTarget.value)}
              />
              <Show when={field().state.meta.isTouched && field().state.meta.errors.length > 0}>
                <p class="fieldset-label text-error">{field().state.meta.errors.join(', ')}</p>
              </Show>
            </fieldset>
          )}
        </form.Field>
      </Show>

      <form.Field
        name="newPassword"
        validators={{
          onChange: ({ value }: { value: string }) => {
            if (!value) return '新密码 是必填项'
            if (value.length < 6) return '密码长度不能少于6位'
          },
        }}
      >
        {(field) => (
          <fieldset class="fieldset mb-3">
            <legend class="fieldset-legend text-[14px]">新密码</legend>
            <input
              type="password"
              value={field().state.value}
              placeholder="请输入新密码"
              class="input w-full"
              onInput={(e) => field().handleChange(e.currentTarget.value)}
            />
            <Show when={field().state.meta.isTouched && field().state.meta.errors.length > 0}>
              <p class="fieldset-label text-error">{field().state.meta.errors.join(', ')}</p>
            </Show>
          </fieldset>
        )}
      </form.Field>

      <form.Field
        name="confirmPassword"
        validators={{
          onChangeListenTo: ['newPassword'],
          onChange: ({ value }: { value: string }) => {
            if (!value) return '确认密码 是必填项'
          },
        }}
      >
        {(field) => (
          <fieldset class="fieldset mb-3">
            <legend class="fieldset-legend text-[14px]">确认密码</legend>
            <input
              type="password"
              value={field().state.value}
              placeholder="请再次输入新密码"
              class="input w-full"
              onInput={(e) => field().handleChange(e.currentTarget.value)}
            />
            <Show when={field().state.meta.isTouched && field().state.meta.errors.length > 0}>
              <p class="fieldset-label text-error">{field().state.meta.errors.join(', ')}</p>
            </Show>
          </fieldset>
        )}
      </form.Field>

      <Show when={error()}>
        <p class="text-error text-sm text-center mb-2">{error()}</p>
      </Show>

      <button
        type="submit"
        class="btn btn-primary w-full"
        disabled={form.state.isSubmitting}
      >
        {form.state.isSubmitting ? '提交中...' : (props.submitText || '确认')}
      </button>
    </form>
  )
}
