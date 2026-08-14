import { TriangleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import styles from './danger-zone.module.scss'

export function DangerZone({
  children,
  description,
  title = 'Zona de perigo',
}: {
  children: ReactNode
  description: string
  title?: string
}) {
  return (
    <section className={styles.root}>
      <header className={styles.header}>
        <span aria-hidden="true" className={styles.icon}>
          <TriangleAlert />
        </span>
        <div>
          <h2>{title}</h2>
          <p>{description}</p>
        </div>
      </header>
      <div className={styles.actions}>{children}</div>
    </section>
  )
}
