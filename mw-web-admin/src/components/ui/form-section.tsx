import { useId, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { Switch } from './switch'
import styles from './form-section.module.scss'

interface FormSectionToggle {
  ariaLabel?: string
  enable?: boolean
  label?: string
  onChange: (state: boolean) => void
  state: boolean
}

export function FormSection({
  children,
  className,
  description,
  title,
  toggle,
}: {
  children: ReactNode
  className?: string
  description?: ReactNode
  title: ReactNode
  toggle?: FormSectionToggle
}) {
  const titleId = useId()

  return (
    <fieldset
      aria-labelledby={titleId}
      className={cn(styles.section, className)}
    >
      <legend className={styles.legend} data-has-toggle={!!toggle}>
        <span id={titleId}>{title}</span>
        {toggle && (
          <>
            <span aria-hidden="true" className={styles.divider} />
            <span className={styles.toggle}>
              {toggle.label && <span>{toggle.label}</span>}
              <Switch
                aria-label={
                  toggle.ariaLabel ??
                  toggle.label ??
                  `Alternar seção ${String(title)}`
                }
                checked={toggle.state}
                disabled={toggle.enable === false}
                onCheckedChange={toggle.onChange}
              />
            </span>
          </>
        )}
      </legend>
      {description && <p>{description}</p>}
      {children}
    </fieldset>
  )
}
