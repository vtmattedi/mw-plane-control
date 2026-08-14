import { cn } from '@/lib/utils'
import { DateSelector } from './date-selector'
import { TimeSelector } from './time-selector'
import styles from './date-time-selector.module.scss'

function parts(value: string): [string, string] {
  const [date = '', time = ''] = value.split('T', 2)
  return [date, time.slice(0, 5)]
}

export function DateTimeSelector({
  value,
  onChange,
  disabled,
  min,
  compact = false,
  className,
}: {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  min?: string
  compact?: boolean
  className?: string
}) {
  const [date, time] = parts(value)
  const [minDate] = parts(min ?? '')

  function update(nextDate: string, nextTime: string) {
    onChange(
      nextDate || nextTime
        ? `${nextDate}${nextTime ? `T${nextTime}` : 'T'}`
        : '',
    )
  }

  return (
    <div className={cn(styles.root, className)}>
      <DateSelector
        compact={compact}
        disabled={disabled}
        min={minDate}
        value={date}
        onChange={(nextDate) => update(nextDate, time)}
      />
      <TimeSelector
        disabled={disabled}
        value={time}
        onChange={(nextTime) => update(date, nextTime)}
      />
    </div>
  )
}
