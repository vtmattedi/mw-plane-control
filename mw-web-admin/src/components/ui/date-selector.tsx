import { Calendar } from './calendar'

function parseDate(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (!match) return null

  const [, year, month, day] = match
  const parsed = new Date(Number(year), Number(month) - 1, Number(day))
  return Number.isNaN(parsed.getTime()) ? null : parsed
}

function formatDate(value: Date): string {
  const year = String(value.getFullYear())
  const month = String(value.getMonth() + 1).padStart(2, '0')
  const day = String(value.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function DateSelector({
  value,
  onChange,
  disabled,
  compact = false,
  min,
  placeholder,
}: {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  compact?: boolean
  min?: string
  placeholder?: string
}) {
  return (
    <Calendar
      compact={compact}
      disabled={disabled}
      min={min ? (parseDate(min) ?? undefined) : undefined}
      placeholder={placeholder}
      value={parseDate(value)}
      onChange={(date) => onChange(formatDate(date))}
    />
  )
}
