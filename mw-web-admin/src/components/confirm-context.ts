import { createContext, useContext } from 'react'

export interface Confirmation {
  title: string
  description: string
  confirmLabel?: string
}

export const ConfirmContext = createContext<
  ((confirmation: Confirmation) => Promise<boolean>) | null
>(null)

export function useConfirm() {
  const confirm = useContext(ConfirmContext)
  if (confirm === null) {
    throw new Error('useConfirm precisa estar dentro de AppConfirmProvider.')
  }

  return confirm
}
