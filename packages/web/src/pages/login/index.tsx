import { createFileRoute, useNavigate } from '@tanstack/solid-router'
import { createForm } from '@tanstack/solid-form'
import { createSignal, Show } from 'solid-js'
import { Form } from '@/components/form'
import { login as loginApi } from '@/api/auth'
import userStore from '@/stores/userStore'

export const Route = createFileRoute('/login/')({
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const [error, setError] = createSignal('')

  const form = createForm(() => ({
    defaultValues: { username: '', password: '' },
    onSubmit: async ({ value }) => {
      setError('')
      try {
        const data = await loginApi(value)
        await userStore.login(data.user, data.access_token, data.refresh_token)
        navigate({ to: '/' })
      } catch (e: any) {
        setError(e?.message || '登录失败，请重试')
      }
    },
  }))

  return (
    <div class="flex items-center justify-center w-screen h-screen bg-base-200">
      <div class="card w-96 bg-base-100 shadow-xl">
        <div class="card-body">
          <div class="text-center mb-2">
            <h1 class="text-3xl font-bold tracking-tight">GOSpeak</h1>
            <p class="text-base-content/50 text-sm mt-1">登录你的账号</p>
          </div>

          <Form
            form={form}
            fields={[
              {
                name: 'username',
                label: '用户名',
                type: 'text',
                placeholder: '请输入用户名',
                required: true,
              },
              {
                name: 'password',
                label: '密码',
                type: 'password',
                placeholder: '请输入密码',
                required: true,
              },
            ]}
            submitButtonText="登录"
          />

          <Show when={error()}>
            <p class="text-error text-sm text-center mt-1">{error()}</p>
          </Show>
        </div>
      </div>
    </div>
  )
}
