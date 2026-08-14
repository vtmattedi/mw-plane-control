import { Clock3, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from './button'
import { Popover, PopoverContent, PopoverTrigger } from './popover'
import { Select } from './select'
import styles from './time-selector.module.scss'

const hours = Array.from({ length: 24 }, (_, index) =>
  String(index).padStart(2, '0'),
)
const minutes = Array.from({ length: 60 }, (_, index) =>
  String(index).padStart(2, '0'),
)

function timeParts(value: string): [string, string] {
  const match = /^(\d{2}):(\d{2})$/.exec(value)
  return match ? [match[1], match[2]] : ['', '']
}

export function TimeSelector({
  value,
  onChange,
  disabled = false,
  placeholder = 'Escolha o horário',
}: {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  placeholder?: string
}) {
  const [open, setOpen] = useState(false)
  const [hour, setHour] = useState(() => timeParts(value)[0])
  const [minute, setMinute] = useState(() => timeParts(value)[1])

  useEffect(() => {
    const [nextHour, nextMinute] = timeParts(value)
    setHour(nextHour)
    setMinute(nextMinute)
  }, [value])

  function update(nextHour: string, nextMinute: string) {
    setHour(nextHour)
    setMinute(nextMinute)
    if (nextHour && nextMinute) onChange(`${nextHour}:${nextMinute}`)
  }

  return (
    <div className={styles.root}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            aria-haspopup="dialog"
            className={styles.trigger}
            disabled={disabled}
            type="button"
            variant="secondary"
          >
            <Clock3 aria-hidden="true" />
            <span>{value || placeholder}</span>
          </Button>
        </PopoverTrigger>

        <PopoverContent
          align="start"
          aria-label="Escolher horário"
          className={styles.popover}
          role="dialog"
        >
          <header>
            <strong>Horário</strong>
            <button
              aria-label="Fechar seletor de horário"
              type="button"
              onClick={() => setOpen(false)}
            >
              <X aria-hidden="true" />
            </button>
          </header>
          <div className={styles.selectors}>
            <label>
              Hora
              <Select
                aria-label="Hora"
                value={hour}
                onChange={(event) => update(event.target.value, minute)}
              >
                <option value="">--</option>
                {hours.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </Select>
            </label>
            <span aria-hidden="true">:</span>
            <label>
              Minuto
              <Select
                aria-label="Minuto"
                value={minute}
                onChange={(event) => update(hour, event.target.value)}
              >
                <option value="">--</option>
                {minutes.map((option) => (
                  <option key={option} value={option}>
                    {option}
                  </option>
                ))}
              </Select>
            </label>
          </div>
          <Button
            disabled={!hour || !minute}
            type="button"
            onClick={() => {
              update(hour, minute)
              setOpen(false)
            }}
          >
            Usar {hour && minute ? `${hour}:${minute}` : 'horário'}
          </Button>
        </PopoverContent>
      </Popover>
    </div>
  )
}
