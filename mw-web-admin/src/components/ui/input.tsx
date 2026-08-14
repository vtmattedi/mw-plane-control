import { Search } from 'lucide-react'
import { forwardRef, type InputHTMLAttributes, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { LgpdIndicator } from './lgpd-indicator'
import styles from './input.module.scss'

export type InputType =
  | 'cpf'
  | 'crp'
  | 'email'
  | 'money'
  | 'number'
  | 'password'
  | 'tel'
  | 'text'
  | 'url'
export type InputVariant = 'default' | 'search' | 'money'

export interface InputProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'children' | 'onChange' | 'type' | 'value'
> {
  indicator?: ReactNode
  indicatorPosition?: 'left' | 'right'
  indicatorSeparator?: boolean
  inputClassName?: string
  lgpd?: boolean
  lgpdNoLink?: boolean
  onChange: (value: string) => void
  type?: InputType
  value: string | number
  variant?: InputVariant
}

function parseMoney(value: string | number) {
  if (typeof value === 'number') return Number.isFinite(value) ? value : 0

  const normalized = value.trim().replace(/[^\d,.-]/g, '')
  if (!normalized) return null

  const decimal = normalized.includes(',')
    ? normalized.replace(/\./g, '').replace(',', '.')
    : normalized
  const parsed = Number(decimal)
  return Number.isFinite(parsed) ? parsed : null
}

function formatMoney(value: string | number) {
  const parsed = parseMoney(value)
  if (parsed == null) return ''

  return new Intl.NumberFormat('pt-BR', {
    maximumFractionDigits: 2,
    minimumFractionDigits: 2,
  }).format(parsed)
}

function formatCpf(value: string | number) {
  const digits = String(value).replace(/\D/g, '').slice(0, 11)

  return digits
    .replace(/^(\d{3})(\d)/, '$1.$2')
    .replace(/^(\d{3})\.(\d{3})(\d)/, '$1.$2.$3')
    .replace(/^(\d{3})\.(\d{3})\.(\d{3})(\d)/, '$1.$2.$3-$4')
}

function formatCrp(value: string | number) {
  const withoutPrefix = String(value).replace(/^\s*CRP\s*/i, '')
  const separatorIndex = withoutPrefix.search(/[/.-]/)
  const regional = (
    separatorIndex >= 0
      ? withoutPrefix.slice(0, separatorIndex)
      : withoutPrefix.slice(0, 2)
  )
    .replace(/\D/g, '')
    .slice(0, 2)
  const registration = (
    separatorIndex >= 0
      ? withoutPrefix.slice(separatorIndex + 1)
      : withoutPrefix.slice(2)
  )
    .replace(/\D/g, '')
    .slice(0, 6)

  return registration
    ? `${regional}/${registration}`
    : separatorIndex >= 0 && regional
      ? `${regional}/`
      : regional
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  {
    className,
    indicator,
    indicatorPosition,
    indicatorSeparator,
    inputClassName,
    lgpd = false,
    lgpdNoLink = false,
    onChange,
    type,
    value,
    variant,
    ...props
  },
  ref,
) {
  const resolvedType = type ?? 'text'
  const resolvedVariant =
    variant ?? (resolvedType === 'money' ? 'money' : 'default')
  const isMoney = resolvedType === 'money' || resolvedVariant === 'money'
  const isCpf = resolvedType === 'cpf'
  const isCrp = resolvedType === 'crp'
  const isSearch = resolvedVariant === 'search'
  const resolvedIndicator =
    indicator ??
    (isSearch ? <Search aria-hidden="true" /> : isMoney ? 'R$' : null)
  const resolvedPosition =
    indicatorPosition ?? (isSearch || isMoney ? 'left' : 'right')
  const resolvedSeparator = indicatorSeparator ?? isMoney

  return (
    <span
      className={cn(styles.root, styles[resolvedVariant], className)}
      data-indicator-position={resolvedPosition}
    >
      {resolvedIndicator != null && (
        <span
          aria-hidden="true"
          className={styles.indicator}
          data-separator={resolvedSeparator || undefined}
        >
          {resolvedIndicator}
        </span>
      )}
      {lgpd && (
        <span className={styles.lgpd}>
          <LgpdIndicator noLink={lgpdNoLink} />
        </span>
      )}
      <input
        {...props}
        ref={ref}
        className={cn(inputClassName, styles.input)}
        inputMode={
          isMoney ? 'decimal' : isCpf || isCrp ? 'numeric' : props.inputMode
        }
        maxLength={props.maxLength ?? (isCpf ? 14 : isCrp ? 9 : undefined)}
        type={
          isMoney || isCpf || isCrp
            ? 'text'
            : isSearch && type == null
              ? 'search'
              : resolvedType
        }
        value={
          isMoney
            ? formatMoney(value)
            : isCpf
              ? formatCpf(value)
              : isCrp
                ? formatCrp(value)
                : value
        }
        onChange={(event) => {
          if (isCpf) {
            onChange(formatCpf(event.target.value))
            return
          }

          if (isCrp) {
            onChange(formatCrp(event.target.value))
            return
          }

          if (!isMoney) {
            onChange(event.target.value)
            return
          }

          const digits = event.target.value.replace(/\D/g, '')
          onChange(digits ? (Number(digits) / 100).toFixed(2) : '')
        }}
      />
    </span>
  )
})
