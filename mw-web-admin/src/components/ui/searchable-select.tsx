import { Check, ChevronsUpDown } from 'lucide-react'
import { useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import { Popover, PopoverContent, PopoverTrigger } from './popover'
import { Input } from './input'
import styles from './searchable-select.module.scss'

export interface SearchableSelectOption {
  disabled?: boolean
  keywords?: string
  label: string
  meta?: string
  value: string
}

interface BaseProps {
  'aria-label': string
  className?: string
  disabled?: boolean
  emptyMessage?: string
  options: SearchableSelectOption[]
  placeholder?: string
  searchPlaceholder?: string
}

interface SingleProps extends BaseProps {
  multiple?: false
  onValueChange: (value: string) => void
  value: string
}

interface MultipleProps extends BaseProps {
  multiple: true
  onValueChange: (value: string[]) => void
  value: string[]
}

export type SearchableSelectProps = SingleProps | MultipleProps

function normalize(value: string) {
  return value
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLocaleLowerCase('pt-BR')
}

export function SearchableSelect(props: SearchableSelectProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const values = props.multiple ? props.value : props.value ? [props.value] : []
  const selected = props.options.filter((option) =>
    values.includes(option.value),
  )
  const visibleOptions = useMemo(() => {
    const term = normalize(search.trim())
    if (!term) return props.options

    return props.options.filter((option) =>
      normalize(`${option.label} ${option.keywords ?? ''}`).includes(term),
    )
  }, [props.options, search])

  function select(value: string) {
    if (props.multiple) {
      props.onValueChange(
        props.value.includes(value)
          ? props.value.filter((item) => item !== value)
          : [...props.value, value],
      )
      return
    }

    props.onValueChange(value)
    setOpen(false)
    setSearch('')
  }

  return (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (!nextOpen) setSearch('')
      }}
    >
      <PopoverTrigger asChild>
        <button
          aria-expanded={open}
          aria-haspopup="listbox"
          aria-label={props['aria-label']}
          className={cn(styles.trigger, props.className)}
          disabled={props.disabled}
          role="combobox"
          type="button"
        >
          <span className={styles.triggerValue}>
            <span className={cn(!selected.length && styles.placeholder)}>
              {selected.length
                ? selected.map((option) => option.label).join(', ')
                : (props.placeholder ?? 'Selecione')}
            </span>
            {selected.length === 1 && selected[0].meta && (
              <small>{selected[0].meta}</small>
            )}
          </span>
          <ChevronsUpDown aria-hidden="true" />
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" className={styles.content}>
        <div className={styles.search}>
          <Input
            aria-label={`Pesquisar em ${props['aria-label']}`}
            autoFocus
            placeholder={props.searchPlaceholder ?? 'Pesquisar…'}
            type="text"
            value={search}
            variant="search"
            onChange={setSearch}
          />
        </div>
        <div
          aria-label={props['aria-label']}
          aria-multiselectable={props.multiple || undefined}
          className={styles.options}
          role="listbox"
        >
          {visibleOptions.map((option) => {
            const isSelected = values.includes(option.value)
            return (
              <button
                aria-label={`${option.label}${
                  option.meta ? `, ${option.meta}` : ''
                }`}
                aria-selected={isSelected}
                className={styles.option}
                disabled={option.disabled}
                key={option.value}
                role="option"
                type="button"
                onClick={() => select(option.value)}
              >
                <span className={styles.optionLabel}>{option.label}</span>
                <span className={styles.optionMeta}>
                  {option.meta && <small>{option.meta}</small>}
                  {isSelected && <Check aria-hidden="true" />}
                </span>
              </button>
            )
          })}
          {!visibleOptions.length && (
            <p className={styles.empty}>
              {props.emptyMessage ?? 'Nenhuma opção encontrada.'}
            </p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
