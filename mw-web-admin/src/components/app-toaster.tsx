import { Toaster } from 'sonner'

export function AppToaster() {
  return (
    <Toaster
      closeButton
      richColors
      position="top-right"
      toastOptions={{
        classNames: {
          toast: 'app-toast',
          title: 'app-toast-title',
          description: 'app-toast-description',
        },
      }}
    />
  )
}
