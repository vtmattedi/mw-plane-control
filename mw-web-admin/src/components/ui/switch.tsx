import type { ButtonHTMLAttributes } from 'react'
import { cn } from '@/lib/utils'
import styles from './switch.module.scss'

interface SwitchProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  'aria-checked' | 'onChange' | 'role'
> {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}

export function Switch({
  checked,
  className,
  disabled,
  onCheckedChange,
  ...props
}: SwitchProps) {
  return (
    <button
      {...props}
      aria-checked={checked}
      className={cn(styles.switch, className)}
      disabled={disabled}
      role="switch"
      type="button"
      onClick={() => onCheckedChange(!checked)}
    >
      <span className={styles.thumb} />
    </button>
  )
}
