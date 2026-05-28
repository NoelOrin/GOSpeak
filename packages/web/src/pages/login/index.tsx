import { createFileRoute, redirect, useNavigate } from '@tanstack/solid-router'
import { createForm } from '@tanstack/solid-form'
import { createSignal, Show, onMount } from 'solid-js'
import { Form } from '@/components/form'
import { login as loginApi, firstChangePassword as firstChangePasswordApi, resetPassword as resetPasswordApi } from '@/api/auth'
import userStore from '@/stores/userStore'
import PasswordChangeForm from '@/components/form/PasswordChangeForm'

export const Route = createFileRoute('/login/')({
  beforeLoad: () => {
    if (userStore.isLoggedIn()) {
      throw redirect({ to: '/' })
    }
  },
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const [error, setError] = createSignal('')
  const [banned, setBanned] = createSignal(false)
  const [showForgotModal, setShowForgotModal] = createSignal(false)

  onMount(() => {
    const params = new URLSearchParams(window.location.search)
    if (params.get('banned') === '1') {
      setBanned(true)
      window.history.replaceState({}, '', '/login')
    }
  })
  const [showChangeModal, setShowChangeModal] = createSignal(false)
  const [loginData, setLoginData] = createSignal<Awaited<ReturnType<typeof loginApi>> | null>(null)

  let forgotDialogRef!: HTMLDialogElement
  let changeDialogRef!: HTMLDialogElement

  const openForgotModal = () => {
    setShowForgotModal(true)
    forgotDialogRef?.showModal()
  }

  const closeForgotModal = () => {
    forgotDialogRef?.close()
    setShowForgotModal(false)
  }

  const openChangeModal = () => {
    setShowChangeModal(true)
    changeDialogRef?.showModal()
  }

  const closeChangeModal = () => {
    changeDialogRef?.close()
    setShowChangeModal(false)
  }

  const form = createForm(() => ({
    defaultValues: { username: '', password: '' },
    onSubmit: async ({ value }) => {
      setError('')
      try {
        const data = await loginApi(value)
        if (data.need_change_password) {
          await userStore.login(data.user, data.access_token, data.refresh_token)
          setLoginData(data)
          openChangeModal()
          return
        }
        await userStore.login(data.user, data.access_token, data.refresh_token)
        navigate({ to: '/' })
      } catch (e: any) {
        if (e?.response?.data?.code === 1015) {
          setBanned(true)
        } else {
          setError(e?.response?.data?.msg || e?.message || '登录失败，请重试')
        }
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

          <div class="text-center mt-1">
            <button class="link link-primary text-sm" onClick={openForgotModal}>
              忘记密码?
            </button>
          </div>

          <Show when={banned()}>
            <div class="alert alert-error mt-2">
              <svg xmlns="http://www.w3.org/2000/svg" class="stroke-current shrink-0 h-5 w-5" fill="none" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
              <span class="text-sm">您的账号已被封禁，无法登录。如有疑问请联系管理员。</span>
            </div>
          </Show>
          <Show when={error()}>
            <p class="text-error text-sm text-center mt-1">{error()}</p>
          </Show>
        </div>
      </div>

      {/* 忘记密码 Modal */}
      <Show when={showForgotModal()}>
        <dialog ref={forgotDialogRef} class="modal" onClose={closeForgotModal}>
          <div class="modal-box">
            <h3 class="font-bold text-lg mb-4">重置密码</h3>
            <PasswordChangeForm
              showUsername={true}
              showOldPassword={false}
              submitText="重置密码"
              onSubmit={async ({ username, newPassword }) => {
                await resetPasswordApi(username!, newPassword)
                closeForgotModal()
              }}
            />
            <div class="modal-action">
              <form method="dialog">
                <button class="btn" onClick={closeForgotModal}>取消</button>
              </form>
            </div>
          </div>
          <form method="dialog" class="modal-backdrop">
            <button onClick={closeForgotModal}>close</button>
          </form>
        </dialog>
      </Show>

      {/* Admin 首次登录强制改密 Modal */}
      <Show when={showChangeModal()}>
        <dialog ref={changeDialogRef} class="modal" onClose={closeChangeModal}>
          <div class="modal-box">
            <h3 class="font-bold text-lg mb-2">修改密码</h3>
            <p class="text-sm text-base-content/60 mb-4">检测到您使用的是默认密码，请修改密码后继续。</p>
            <PasswordChangeForm
              showOldPassword={false}
              showName={true}
              submitText="修改密码"
              onSubmit={async ({ newPassword, name }) => {
                const updatedUser = await firstChangePasswordApi(newPassword, name)
                const data = loginData()
                if (data) {
                  await userStore.login(updatedUser, data.access_token, data.refresh_token)
                }
                closeChangeModal()
                navigate({ to: '/' })
              }}
            />
          </div>
          <form method="dialog" class="modal-backdrop">
            <button>close</button>
          </form>
        </dialog>
      </Show>
    </div>
  )
}
