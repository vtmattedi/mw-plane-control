import type { ReactNode } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface EntityCreationDialogProps {
  children: ReactNode
  description: string
  onOpenChange: (open: boolean) => void
  open: boolean
  preventClose?: boolean
  title: string
}

export function EntityCreationDialog({
  children,
  description,
  onOpenChange,
  open,
  preventClose = false,
  title,
}: EntityCreationDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && preventClose) return
        onOpenChange(nextOpen)
      }}
    >
      <DialogContent
        closeDisabled={preventClose}
        onEscapeKeyDown={(event) => {
          if (preventClose) event.preventDefault()
        }}
        onPointerDownOutside={(event) => {
          if (preventClose) event.preventDefault()
        }}
      >
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {children}
      </DialogContent>
    </Dialog>
  )
}
