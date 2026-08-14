import * as SelectPrimitive from '@radix-ui/react-select'
import { Check, ChevronDown, ChevronUp } from 'lucide-react'
import {
  Children,
  Fragment,
  forwardRef,
  isValidElement,
  type ChangeEvent,
  type ChangeEventHandler,
  type ReactNode,
} from 'react'
import { cn } from '@/lib/utils'
import styles from './select.module.scss'

const emptyValue = '__mantra_empty_select_value__'

interface SelectOption {
  disabled: boolean
  label: ReactNode
  value: string
}

function optionsFrom(children: ReactNode): SelectOption[] {
  return Children.toArray(children).flatMap((child): SelectOption[] => {
    if (!isValidElement(child)) return []
    if (child.type === Fragment) {
      return optionsFrom((child.props as { children?: ReactNode }).children)
    }
    if (child.type !== 'option') return []

    const props = child.props as {
      children?: ReactNode
      disabled?: boolean
      value?: string | number
    }
    return [
      {
        disabled: props.disabled ?? false,
        label: props.children,
        value: String(props.value ?? ''),
      },
    ]
  })
}

export interface SelectProps {
  'aria-describedby'?: string
  'aria-invalid'?: boolean
  'aria-label'?: string
  'aria-labelledby'?: string
  children: ReactNode
  className?: string
  disabled?: boolean
  id?: string
  name?: string
  onChange?: ChangeEventHandler<HTMLSelectElement>
  required?: boolean
  value?: string | number
}

export function SelectColorOption({
  children,
  color,
}: {
  children: ReactNode
  color: string
}) {
  return (
    <span className={styles.colorOption}>
      <span
        aria-hidden="true"
        className={styles.colorSwatch}
        style={{ backgroundColor: color }}
      />
      <span>{children}</span>
    </span>
  )
}

export const Select = forwardRef<HTMLButtonElement, SelectProps>(
  function Select(
    {
      children,
      className,
      disabled,
      id,
      name,
      onChange,
      required,
      value = '',
      ...aria
    },
    ref,
  ) {
    const options = optionsFrom(children)
    const selectedValue = String(value) || emptyValue

    function change(nextValue: string) {
      const actualValue = nextValue === emptyValue ? '' : nextValue
      const target = { name: name ?? '', value: actualValue }
      onChange?.({
        currentTarget: target,
        target,
      } as ChangeEvent<HTMLSelectElement>)
    }

    return (
      <SelectPrimitive.Root
        disabled={disabled}
        name={name}
        required={required}
        value={selectedValue}
        onValueChange={change}
      >
        <SelectPrimitive.Trigger
          {...aria}
          aria-required={required || undefined}
          className={cn(styles.trigger, className)}
          id={id}
          ref={ref}
        >
          <SelectPrimitive.Value />
          <SelectPrimitive.Icon asChild>
            <ChevronDown aria-hidden="true" className={styles.icon} />
          </SelectPrimitive.Icon>
        </SelectPrimitive.Trigger>
        <SelectPrimitive.Portal>
          <SelectPrimitive.Content
            className={styles.content}
            data-mantra-select-content=""
            position="popper"
            sideOffset={5}
          >
            <SelectPrimitive.ScrollUpButton className={styles.scrollButton}>
              <ChevronUp aria-hidden="true" />
            </SelectPrimitive.ScrollUpButton>
            <SelectPrimitive.Viewport className={styles.viewport}>
              {options.map((option) => (
                <SelectPrimitive.Item
                  className={styles.item}
                  disabled={option.disabled}
                  key={option.value || emptyValue}
                  value={option.value || emptyValue}
                >
                  <SelectPrimitive.ItemText>
                    {option.label}
                  </SelectPrimitive.ItemText>
                  <SelectPrimitive.ItemIndicator className={styles.indicator}>
                    <Check aria-hidden="true" />
                  </SelectPrimitive.ItemIndicator>
                </SelectPrimitive.Item>
              ))}
            </SelectPrimitive.Viewport>
            <SelectPrimitive.ScrollDownButton className={styles.scrollButton}>
              <ChevronDown aria-hidden="true" />
            </SelectPrimitive.ScrollDownButton>
          </SelectPrimitive.Content>
        </SelectPrimitive.Portal>
      </SelectPrimitive.Root>
    )
  },
)
