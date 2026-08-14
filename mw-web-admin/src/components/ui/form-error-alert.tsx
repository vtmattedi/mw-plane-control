import { CircleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import styles from './form-error-alert.module.scss'
import { toHtml } from '@/lib/utils'

export function FormErrorAlert({
  message,
  tone = 'error',
  role = 'alert',
}: {
  message: ReactNode
  tone?: 'error' | 'warning'
  /** Static, always-visible notes should use "note" instead of the default
   * "alert" — "alert" is for transient messages that need an immediate
   * screen-reader announcement. */
  role?: 'alert' | 'note'
}) {
  return (
    <div
      className={`${styles.alert} ${tone === 'warning' ? styles.warning : ''}`}
      role={role}
    >
      <p>
        <CircleAlert aria-hidden="true" className={styles.icon} />
        {typeof message === 'string' ? toHtml(message) : message}
      </p>
    </div>
  )
}
