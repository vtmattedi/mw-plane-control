import { useId, type CSSProperties } from 'react'
import { Input } from './input'
import styles from './split-input.module.scss'
import { cn } from '@/lib/utils'

export function SplitInput({
  disabled = false,
  label = 'Percentual da clínica',
  onChange,
  value,
  noHeader = false,
  className,
  form,
}: {
  disabled?: boolean
  label?: string
  onChange: (value: number) => void
  value: number
  noHeader?: boolean
  className?: string
  form?: string
}) {
  const id = useId()
  const percentage = Math.round(Math.max(0, Math.min(1, value)) * 10000) / 100
  const update = (raw: number) =>
    Number.isFinite(raw) && onChange(Math.max(0, Math.min(100, raw)) / 100)

  return (
    <div className={cn(styles.root, className)}>
      {noHeader ? null : (
        <div className={styles.heading}>
          <label htmlFor={id}>{label}</label>
          <output htmlFor={id}>
            {percentage.toLocaleString('pt-BR')}% clínica ·{' '}
            {(100 - percentage).toLocaleString('pt-BR')}% profissional
          </output>
        </div>
      )}
      <div className={styles.controls}>
        <input
          aria-label={`${label}: controle deslizante`}
          disabled={disabled}
          form={form}
          max={100}
          min={0}
          step={1}
          type="range"
          value={percentage}
          style={{ '--split-percentage': `${percentage}%` } as CSSProperties}
          onChange={(event) => update(Number(event.target.value))}
        />
        <Input
          aria-label={`${label}: valor`}
          className={styles.percentage}
          id={id}
          disabled={disabled}
          form={form}
          indicator="%"
          indicatorPosition="right"
          indicatorSeparator={true}
          inputMode="decimal"
          max={100}
          min={0}
          step={1}
          type="number"
          value={percentage}
          onChange={(nextValue) => update(Number(nextValue))}
        />
      </div>
    </div>
  )
}
