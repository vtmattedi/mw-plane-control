import {
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  ChevronsUpDown,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import { Button } from './button'
import { Popover, PopoverContent, PopoverTrigger } from './popover'
import styles from './calendar.module.scss'

const weekdays = ['Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb', 'Dom']

function sentenceCase(value: string) {
  return value.charAt(0).toLocaleUpperCase('pt-BR') + value.slice(1)
}

function compactDate(value: Date) {
  const month = value
    .toLocaleDateString('pt-BR', { month: 'short' })
    .replace('.', '')
  return `${String(value.getDate()).padStart(2, '0')}/${sentenceCase(month)}`
}

function sameDay(left: Date, right: Date): boolean {
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  )
}

function monthDays(month: Date): Date[] {
  const first = new Date(month.getFullYear(), month.getMonth(), 1)
  const mondayOffset = (first.getDay() + 6) % 7
  const start = new Date(first)
  start.setDate(first.getDate() - mondayOffset)

  return Array.from({ length: 42 }, (_, index) => {
    const day = new Date(start)
    day.setDate(start.getDate() + index)
    return day
  })
}

export function Calendar({
  value,
  onChange,
  disabled = false,
  compact = false,
  min,
  placeholder = 'Escolha uma data',
}: {
  value: Date | null
  onChange: (date: Date) => void
  disabled?: boolean
  compact?: boolean
  min?: Date
  placeholder?: string
}) {
  const initialDate = value ?? new Date()
  const [open, setOpen] = useState(false)
  const [visibleMonth, setVisibleMonth] = useState(
    () => new Date(initialDate.getFullYear(), initialDate.getMonth(), 1),
  )
  const days = useMemo(() => monthDays(visibleMonth), [visibleMonth])
  const today = useMemo(() => new Date(), [])

  function changeOpen(nextOpen: boolean) {
    setOpen(nextOpen)
    if (nextOpen) {
      const activeDate = value ?? new Date()
      setVisibleMonth(
        new Date(activeDate.getFullYear(), activeDate.getMonth(), 1),
      )
    }
  }

  function moveMonth(direction: -1 | 1) {
    setVisibleMonth(
      (current) =>
        new Date(current.getFullYear(), current.getMonth() + direction, 1),
    )
  }

  return (
    <div className={styles.root}>
      <Popover open={open} onOpenChange={changeOpen}>
        <PopoverTrigger asChild>
          <Button
            aria-label={
              value
                ? value.toLocaleDateString('pt-BR', { dateStyle: 'long' })
                : placeholder
            }
            aria-haspopup="dialog"
            className={cn(
              styles.trigger,
              compact && styles.compactTrigger,
              'w-full',
            )}
            disabled={disabled}
            type="button"
            variant="secondary"
          >
            <CalendarDays aria-hidden="true" />
            <span>
              {value
                ? compact
                  ? compactDate(value)
                  : value.toLocaleDateString('pt-BR', {
                      dateStyle: 'long',
                    })
                : placeholder}
            </span>
            <ChevronsUpDown aria-hidden="true" />
          </Button>
        </PopoverTrigger>

        <PopoverContent
          aria-label="Escolher data"
          className={styles.popover}
          role="dialog"
        >
          <header className={styles.header}>
            <button
              aria-label="Mês anterior"
              className={styles.navigation}
              type="button"
              onClick={() => moveMonth(-1)}
            >
              <ChevronLeft aria-hidden="true" />
            </button>
            <strong>
              {sentenceCase(
                visibleMonth.toLocaleDateString('pt-BR', {
                  month: 'long',
                  year: 'numeric',
                }),
              )}
            </strong>
            <button
              aria-label="Próximo mês"
              className={styles.navigation}
              type="button"
              onClick={() => moveMonth(1)}
            >
              <ChevronRight aria-hidden="true" />
            </button>
          </header>

          <div className={styles.grid}>
            {weekdays.map((weekday) => (
              <span className={styles.weekday} key={weekday}>
                {weekday}
              </span>
            ))}
            {days.map((day) => {
              const selected = value !== null && sameDay(day, value)
              const outside = day.getMonth() !== visibleMonth.getMonth()
              const unavailable =
                min !== undefined &&
                new Date(
                  day.getFullYear(),
                  day.getMonth(),
                  day.getDate(),
                ).getTime() <
                  new Date(
                    min.getFullYear(),
                    min.getMonth(),
                    min.getDate(),
                  ).getTime()
              return (
                <button
                  aria-current={sameDay(day, today) ? 'date' : undefined}
                  aria-label={day.toLocaleDateString('pt-BR', {
                    dateStyle: 'full',
                  })}
                  aria-selected={selected}
                  className={styles.day}
                  data-outside={outside || undefined}
                  data-selected={selected || undefined}
                  disabled={unavailable}
                  key={day.toISOString()}
                  type="button"
                  onClick={() => {
                    onChange(day)
                    setOpen(false)
                  }}
                >
                  {day.getDate()}
                </button>
              )
            })}
          </div>

          <Button
            className={styles.today}
            type="button"
            variant="secondary"
            onClick={() => {
              const next = new Date()
              onChange(next)
              setOpen(false)
            }}
          >
            Hoje
          </Button>
        </PopoverContent>
      </Popover>
    </div>
  )
}
