import { HexColorPicker } from 'react-colorful'
import { useId, useState, type CSSProperties } from 'react'
import { cn } from '@/lib/utils'
import { Popover, PopoverContent, PopoverTrigger } from './popover'
import styles from './color-input.module.scss'

const COMPLETE_HEX_COLOR = /^#[0-9a-fA-F]{6}$/
const EDITABLE_HEX_COLOR = /^#?[0-9a-fA-F]{0,6}$/
const FALLBACK_COLOR = '#4B7891'

function editableColor(input: string, fallback: string) {
  if (!EDITABLE_HEX_COLOR.test(input)) return fallback
  if (input === '') return ''
  return input.startsWith('#') ? input.toUpperCase() : `#${input.toUpperCase()}`
}

export function ColorInput({
  className,
  disabled = false,
  hideLabel = false,
  label,
  onChange,
  value,
}: {
  className?: string
  disabled?: boolean
  hideLabel?: boolean
  label: string
  onChange: (value: string) => void
  value: string
}) {
  const id = useId()
  const [open, setOpen] = useState(false)
  const colorValue = COMPLETE_HEX_COLOR.test(value) ? value : FALLBACK_COLOR

  return (
    <div className={cn(styles.root, className)}>
      <label
        className={hideLabel ? styles.labelHidden : undefined}
        htmlFor={id}
      >
        {label}
      </label>
      <span className={styles.control}>
        <Popover
          open={open}
          onOpenChange={(nextOpen) => {
            if (!disabled) setOpen(nextOpen)
          }}
        >
          <PopoverTrigger asChild>
            <button
              aria-label={`Escolher ${label.toLowerCase()}`}
              className={styles.swatch}
              disabled={disabled}
              style={{ '--selected-color': colorValue } as CSSProperties}
              type="button"
            />
          </PopoverTrigger>
          <PopoverContent align="start" className={styles.popover}>
            <HexColorPicker
              color={colorValue}
              onChange={(nextColor) => onChange(nextColor.toUpperCase())}
            />
          </PopoverContent>
        </Popover>
        <input
          aria-label={`${label}: valor hexadecimal`}
          className={styles.value}
          disabled={disabled}
          id={id}
          inputMode="text"
          maxLength={7}
          placeholder="Cor em hexadecimal"
          spellCheck={false}
          value={value}
          onBlur={(event) => {
            if (COMPLETE_HEX_COLOR.test(event.currentTarget.value)) {
              onChange(event.currentTarget.value.toUpperCase())
            }
          }}
          onChange={(event) =>
            onChange(editableColor(event.target.value, value))
          }
        />
      </span>
    </div>
  )
}
