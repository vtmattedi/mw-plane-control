import { Eye, EyeOff } from 'lucide-react'
import { useState, type InputHTMLAttributes } from 'react'
import styles from './password-field.module.scss'

interface PasswordFieldProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'type'
> {
  error?: string
  label: string
  name: string
}

export function PasswordField({
  'aria-describedby': describedBy,
  error,
  id,
  label,
  name,
  ...inputProps
}: PasswordFieldProps) {
  const [visible, setVisible] = useState(false)
  const inputId = id ?? name
  const errorId = `${inputId}-error`

  return (
    <div className={styles.field}>
      <label className={styles.label} htmlFor={inputId}>
        {label}
      </label>
      <div className={styles.control}>
        <input
          {...inputProps}
          aria-describedby={
            [describedBy, error ? errorId : undefined]
              .filter(Boolean)
              .join(' ') || undefined
          }
          aria-invalid={error ? true : undefined}
          id={inputId}
          name={name}
          type={visible ? 'text' : 'password'}
        />
        <button
          aria-controls={inputId}
          aria-label={
            visible
              ? `Ocultar ${label.toLowerCase()}`
              : `Mostrar ${label.toLowerCase()}`
          }
          className={styles.toggle}
          type="button"
          onClick={() => setVisible((current) => !current)}
        >
          {visible ? <EyeOff aria-hidden="true" /> : <Eye aria-hidden="true" />}
        </button>
      </div>
      {error && (
        <p className={styles.error} id={errorId}>
          {error}
        </p>
      )}
    </div>
  )
}
