import type { ReactNode } from 'react'
import styles from './entity-detail.module.scss'

export interface EntitySummaryField {
  label: string
  value: ReactNode
}

export function EntitySummaryCard({
  actions,
  badge,
  fields,
  identity,
  subtitle,
  title,
}: {
  actions?: ReactNode
  badge?: ReactNode
  fields: EntitySummaryField[]
  identity?: ReactNode
  subtitle: ReactNode
  title: ReactNode
}) {
  return (
    <section className={styles.summary} aria-labelledby="entity-summary-title">
      <div className={styles.heading}>
        <div className={styles.identity}>
          {identity}
          <div>
            <p>Resumo do cadastro</p>
            <div className={styles.title}>
              <h1 id="entity-summary-title">{title}</h1>
              {badge}
            </div>
            <p>{subtitle}</p>
          </div>
        </div>
        {actions}
      </div>
      <dl className={styles.fields}>
        {fields.map((field) => (
          <div className={styles.field} key={field.label}>
            <dt>{field.label}</dt>
            <dd>{field.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}

export function EntityRelationships({
  children,
  description,
  tabs,
  title,
}: {
  children: ReactNode
  description: string
  tabs?: ReactNode
  title: string
}) {
  return (
    <section className={styles.relations}>
      <div className={styles.relationsHeader}>
        <div>
          <h2>{title}</h2>
          <p>{description}</p>
        </div>
      </div>
      {tabs}
      {children}
    </section>
  )
}
