import { createFileRoute, redirect, useNavigate } from '@tanstack/solid-router'
import { createForm } from '@tanstack/solid-form'
import { createSignal, Show } from 'solid-js'
import { Form } from '@/components/form'
import { login as loginApi, changePassword as changePasswordApi, resetPassword as resetPasswordApi } from '@/api/auth'
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
  const [showForgotModal, setShowForgotModal] = createSignal(false)
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
          setLoginData(data)
          openChangeModal()
          return
        }
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

          <div class="text-center mt-1">
            <button class="link link-primary text-sm" onClick={openForgotModal}>
              忘记密码?
            </button>
          </div>

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
              showOldPassword={true}
              submitText="修改密码"
              onSubmit={async ({ oldPassword, newPassword }) => {
                await changePasswordApi(oldPassword!, newPassword)
                const data = loginData()
                if (data) {
                  await userStore.login(data.user, data.access_token, data.refresh_token)
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
