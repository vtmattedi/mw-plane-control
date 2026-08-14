import * as React from 'react'
import { ImageIcon } from 'lucide-react'

import {
  FileTextIcon as FileText,
  type FileTextIconHandle,
} from '@/components/animated-icons/file-text'
import {
  CheckIcon as Check,
  type CheckIconHandle,
} from '@/components/animated-icons/check'
import {
  CloudUploadIcon as UploadCloud,
  type CloudUploadIconHandle,
} from '@/components/animated-icons/cloud-upload'

import { cn } from '@/lib/utils'

type FileInputProps = Omit<
  React.ComponentProps<'input'>,
  'type' | 'className'
> & {
  className?: string

  /** Indica se já existe um arquivo salvo no servidor */
  hasFile?: boolean

  /** Metadados do arquivo existente */
  existingFileName?: string
  existingFileUrl?: string
  existingFileType?: string
  existingFileSize?: number

  label?: string
  description?: string
}

function formatFileSize(bytes?: number) {
  if (bytes == null) return null

  if (bytes < 1024) {
    return `${bytes} B`
  }

  if (bytes < 1024 ** 2) {
    return `${(bytes / 1024).toFixed(1)} KB`
  }

  if (bytes < 1024 ** 3) {
    return `${(bytes / 1024 ** 2).toFixed(1)} MB`
  }

  return `${(bytes / 1024 ** 3).toFixed(1)} GB`
}

function getFileTypeLabel(type?: string, name?: string) {
  if (type) return type

  const extension = name?.split('.').pop()

  if (extension) {
    return extension.toUpperCase()
  }

  return 'Arquivo'
}

export function FileInput({
  className,

  hasFile = false,
  existingFileName,
  existingFileUrl,
  existingFileType,
  existingFileSize,

  label = 'Escolha um arquivo ou arraste-o aqui',
  description,

  disabled,
  multiple,
  onChange,

  ...inputProps
}: FileInputProps) {
  const inputRef = React.useRef<HTMLInputElement>(null)

  const uploadIconRef = React.useRef<CloudUploadIconHandle>(null)
  const fileIconRef = React.useRef<FileTextIconHandle>(null)
  const checkIconRef = React.useRef<CheckIconHandle>(null)

  const [isDragging, setIsDragging] = React.useState(false)
  const [selectedFiles, setSelectedFiles] = React.useState<File[]>([])
  const [previewUrl, setPreviewUrl] = React.useState<string>()

  const selectedFile = selectedFiles[0]

  React.useEffect(() => {
    if (!selectedFile?.type.startsWith('image/')) {
      setPreviewUrl(undefined)
      return
    }

    const url = URL.createObjectURL(selectedFile)

    setPreviewUrl(url)

    return () => {
      URL.revokeObjectURL(url)
    }
  }, [selectedFile])

  const hasSelectedFile = selectedFiles.length > 0
  const hasAnyFile = hasFile || hasSelectedFile

  const fileName = selectedFile?.name ?? existingFileName
  const fileType = selectedFile?.type || existingFileType
  const fileSize = selectedFile?.size ?? existingFileSize

  const isImage = selectedFile
    ? selectedFile.type.startsWith('image/')
    : (existingFileType?.startsWith('image/') ?? false)

  const imageUrl = previewUrl ?? existingFileUrl

  const startIconAnimation = () => {
    uploadIconRef.current?.startAnimation()
    fileIconRef.current?.startAnimation()
    checkIconRef.current?.startAnimation()
  }

  const stopIconAnimation = () => {
    uploadIconRef.current?.stopAnimation()
    fileIconRef.current?.stopAnimation()
    checkIconRef.current?.stopAnimation()
  }

  const openFilePicker = () => {
    if (disabled) return

    inputRef.current?.click()
  }

  const handleInputChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setSelectedFiles(Array.from(event.target.files ?? []))

    onChange?.(event)
  }

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (disabled) return

    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      openFilePicker()
    }
  }

  const handleDragOver = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault()

    if (!disabled) {
      setIsDragging(true)
      uploadIconRef.current?.startAnimation()
    }
  }

  const handleDragLeave = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault()

    if (
      event.relatedTarget &&
      event.currentTarget.contains(event.relatedTarget as Node)
    ) {
      return
    }

    setIsDragging(false)
    uploadIconRef.current?.stopAnimation()
  }

  const handleDrop = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    setIsDragging(false)

    uploadIconRef.current?.stopAnimation()

    if (disabled || !inputRef.current) return

    const droppedFiles = Array.from(event.dataTransfer.files)

    if (!droppedFiles.length) return

    const files = multiple ? droppedFiles : [droppedFiles[0]]

    const dataTransfer = new DataTransfer()

    files.forEach((file) => {
      dataTransfer.items.add(file)
    })

    inputRef.current.files = dataTransfer.files

    inputRef.current.dispatchEvent(
      new Event('change', {
        bubbles: true,
      }),
    )
  }

  return (
    <div
      role="button"
      tabIndex={disabled ? -1 : 0}
      aria-disabled={disabled}
      onClick={openFilePicker}
      onKeyDown={handleKeyDown}
      onMouseEnter={startIconAnimation}
      onMouseLeave={stopIconAnimation}
      onDragEnter={handleDragOver}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      className={cn(
        'group relative flex min-h-24 w-full cursor-pointer',
        'items-center justify-center overflow-hidden',
        'border-border rounded-xl border border-dashed',
        'bg-background p-4',
        'transition-colors',
        'hover:border-foreground/30 hover:bg-muted/40',

        'focus-visible:outline-none',
        'focus-visible:ring-2',
        'focus-visible:ring-ring',
        'focus-visible:ring-offset-2',

        isDragging && 'border-primary bg-primary/5',

        hasAnyFile && !isDragging && 'border-solid',

        disabled && 'pointer-events-none cursor-not-allowed opacity-50',

        className,
      )}
    >
      <input
        {...inputProps}
        ref={inputRef}
        type="file"
        multiple={multiple}
        disabled={disabled}
        onChange={handleInputChange}
        className="sr-only"
        tabIndex={-1}
      />

      {isDragging ? (
        <DropState iconRef={uploadIconRef} />
      ) : hasAnyFile ? (
        isImage && imageUrl ? (
          <ImagePreview
            src={imageUrl}
            fileName={fileName}
            checkIconRef={checkIconRef}
          />
        ) : (
          <FileMetadata
            name={fileName}
            type={fileType}
            size={fileSize}
            iconRef={fileIconRef}
          />
        )
      ) : (
        <EmptyState
          label={label}
          description={description}
          iconRef={uploadIconRef}
        />
      )}
    </div>
  )
}

function EmptyState({
  label,
  description,
  iconRef,
}: {
  label: string
  description?: string
  iconRef: React.RefObject<CloudUploadIconHandle | null>
}) {
  return (
    <div className="flex flex-col items-center gap-2 text-center">
      <div className="bg-muted/50 flex size-9 items-center justify-center rounded-lg border">
        <UploadCloud ref={iconRef} size={16} />
      </div>

      <div className="space-y-0.5">
        <p className="text-sm font-medium">{label}</p>

        {description && (
          <p className="text-muted-foreground text-xs">{description}</p>
        )}
      </div>
    </div>
  )
}

function DropState({
  iconRef,
}: {
  iconRef: React.RefObject<CloudUploadIconHandle | null>
}) {
  return (
    <div className="flex flex-col items-center gap-2 text-center">
      <div className="bg-primary/10 flex size-9 scale-110 items-center justify-center rounded-lg border">
        <UploadCloud ref={iconRef} size={16} />
      </div>

      <p className="text-sm font-medium">Solte o arquivo aqui</p>
    </div>
  )
}

function ImagePreview({
  src,
  fileName,
  checkIconRef,
}: {
  src: string
  fileName?: string
  checkIconRef: React.RefObject<CheckIconHandle | null>
}) {
  return (
    <div className="flex w-full items-center gap-3">
      <div className="bg-muted relative size-24 shrink-0 overflow-hidden rounded-lg border">
        <img src={src} alt="" className="size-full object-cover" />

        <div className="bg-background/90 absolute top-1 right-1 flex size-5 items-center justify-center rounded-full shadow-sm backdrop-blur-sm">
          <Check ref={checkIconRef} size={12} />
        </div>
      </div>

      <div className="min-w-0 flex-1 text-left">
        <div className="text-muted-foreground mb-0.5 flex items-center gap-1.5 text-xs">
          <ImageIcon className="size-3.5" />
          Imagem
        </div>

        <p className="truncate text-sm font-medium">
          {fileName ?? 'Imagem atual'}
        </p>

        <p className="text-muted-foreground mt-0.5 text-xs">
          Clique ou arraste uma imagem para substituir
        </p>
      </div>
    </div>
  )
}

function FileMetadata({
  name,
  type,
  size,
  iconRef,
}: {
  name?: string
  type?: string
  size?: number
  iconRef: React.RefObject<FileTextIconHandle | null>
}) {
  const formattedSize = formatFileSize(size)

  return (
    <div className="flex w-full items-center gap-3">
      <div className="bg-muted/50 flex size-12 shrink-0 items-center justify-center rounded-lg border">
        <FileText ref={iconRef} size={20} />
      </div>

      <div className="min-w-0 flex-1 text-left">
        <p className="truncate text-sm font-medium">
          {name ?? 'Arquivo enviado'}
        </p>

        <div className="text-muted-foreground mt-0.5 flex flex-wrap items-center gap-x-2 text-xs">
          <span>{getFileTypeLabel(type, name)}</span>

          {formattedSize && (
            <>
              <span aria-hidden>•</span>
              <span>{formattedSize}</span>
            </>
          )}
        </div>

        <p className="text-muted-foreground mt-1 text-xs">
          Clique ou arraste um arquivo para substituir
        </p>
      </div>
    </div>
  )
}
