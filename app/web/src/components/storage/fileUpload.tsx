import { type Component, type JSX, createSignal, Show } from 'solid-js'
import { useUpload, type UploadResult } from '@/hooks/useUpload'
import { showToast } from 'solid-notifications'

interface FileUploadProps {
  category: string
  accept?: string
  maxSizeMB?: number
  onUploadComplete?: (result: UploadResult) => void
  children?: JSX.Element
  class?: string
}

const FileUpload: Component<FileUploadProps> = (props) => {
  const { upload, uploading, progress, error } = useUpload(props.category)
  const [dragOver, setDragOver] = createSignal(false)
  let inputRef: HTMLInputElement | undefined

  const maxBytes = (props.maxSizeMB || 5) * 1024 * 1024

  const handleFile = async (file: File) => {
    if (file.size > maxBytes) {
      showToast(`文件大小超过 ${props.maxSizeMB || 5}MB 限制`, { type: 'error' })
      return
    }

    try {
      const result = await upload(file)
      props.onUploadComplete?.(result)
    } catch (e) {
      const msg = e instanceof Error ? e.message : '上传失败'
      showToast(msg, { type: 'error' })
    }
  }

  const onFileSelected: JSX.EventHandler<HTMLInputElement, Event> = (e) => {
    const files = e.currentTarget.files
    if (files && files.length > 0) {
      handleFile(files[0])
    }
    // 重置 input 以允许选择同一文件
    if (inputRef) inputRef.value = ''
  }

  const onDrop: JSX.EventHandler<HTMLDivElement, DragEvent> = (e) => {
    e.preventDefault()
    setDragOver(false)
    const files = e.dataTransfer?.files
    if (files && files.length > 0) {
      handleFile(files[0])
    }
  }

  const onDragOver: JSX.EventHandler<HTMLDivElement, DragEvent> = (e) => {
    e.preventDefault()
    setDragOver(true)
  }

  const onDragLeave: JSX.EventHandler<HTMLDivElement, DragEvent> = (e) => {
    e.preventDefault()
    setDragOver(false)
  }

  return (
    <div
      class={`flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-6 transition-colors cursor-pointer ${dragOver() ? 'border-primary bg-primary/10' : 'border-base-300 hover:border-primary/50'} ${props.class || ''}`}
      onClick={() => inputRef?.click()}
      onDrop={onDrop}
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
    >
      <input
        ref={inputRef}
        type="file"
        accept={props.accept || 'image/jpeg,image/png,image/gif,image/webp'}
        class="hidden"
        onChange={onFileSelected}
        disabled={uploading()}
      />

      <Show when={uploading()}>
        <div class="flex flex-col items-center gap-2 w-full">
          <span class="loading loading-spinner loading-lg text-primary" />
          <div class="w-full bg-base-200 rounded-full h-2">
            <div
              class="bg-primary h-2 rounded-full transition-all duration-300"
              style={{ width: `${progress()}%` }}
            />
          </div>
          <span class="text-sm text-base-content/60">上传中... {progress()}%</span>
        </div>
      </Show>

      <Show when={!uploading()}>
        {props.children || (
          <div class="flex flex-col items-center gap-2 text-base-content/60">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
            </svg>
            <span class="text-sm">点击或拖拽文件到此处上传</span>
          </div>
        )}
      </Show>

      <Show when={error()}>
        <p class="text-error text-sm mt-2">{error()}</p>
      </Show>
    </div>
  )
}

export default FileUpload
