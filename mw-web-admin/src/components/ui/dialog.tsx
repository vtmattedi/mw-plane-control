import * as DialogPrimitive from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import {
  forwardRef,
  type ComponentPropsWithoutRef,
  type ReactNode,
} from 'react'
import { cn } from '@/lib/utils'
import styles from './dialog.module.scss'

export const Dialog = DialogPrimitive.Root
export const DialogTrigger = DialogPrimitive.Trigger
export const DialogClose = DialogPrimitive.Close

interface DialogContentProps extends ComponentPropsWithoutRef<
  typeof DialogPrimitive.Content
> {
  closeDisabled?: boolean
}

export const DialogContent = forwardRef<HTMLDivElement, DialogContentProps>(
  function DialogContent(
    { children, className, closeDisabled = false, ...props },
    ref,
  ) {
    return (
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className={styles.overlay} />
        <DialogPrimitive.Content
          className={cn(styles.content, className)}
          ref={ref}
          {...props}
        >
          {children}
          <DialogPrimitive.Close
            aria-label="Fechar"
            className={styles.close}
            disabled={closeDisabled}
            type="button"
          >
            <X aria-hidden="true" />
          </DialogPrimitive.Close>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    )
  },
)

export function DialogHeader({ children }: { children: ReactNode }) {
  return <header className={styles.header}>{children}</header>
}

export function DialogFooter({ children }: { children: ReactNode }) {
  return <footer className={styles.footer}>{children}</footer>
}

export const DialogTitle = forwardRef<
  HTMLHeadingElement,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(function DialogTitle({ className, ...props }, ref) {
  return (
    <DialogPrimitive.Title
      className={cn(styles.title, className)}
      ref={ref}
      {...props}
    />
  )
})

export const DialogDescription = forwardRef<
  HTMLParagraphElement,
  ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(function DialogDescription({ className, ...props }, ref) {
  return (
    <DialogPrimitive.Description
      className={cn(styles.description, className)}
      ref={ref}
      {...props}
    />
  )
})
