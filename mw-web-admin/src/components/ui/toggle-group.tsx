import * as ToggleGroupPrimitive from '@radix-ui/react-toggle-group'
import { forwardRef, type ComponentPropsWithoutRef } from 'react'
import { cn } from '@/lib/utils'

export const ToggleGroup = forwardRef<
  HTMLDivElement,
  Omit<ToggleGroupPrimitive.ToggleGroupSingleProps, 'type'>
>(function ToggleGroup({ className, ...props }, ref) {
  return (
    <ToggleGroupPrimitive.Root
      className={cn(
        'border-border bg-muted inline-flex items-center gap-1 rounded-xl border p-1',
        className,
      )}
      ref={ref}
      data-slot="toggle-group"
      type="single"
      {...props}
    />
  )
})

export const ToggleGroupItem = forwardRef<
  HTMLButtonElement,
  ComponentPropsWithoutRef<typeof ToggleGroupPrimitive.Item>
>(function ToggleGroupItem({ className, ...props }, ref) {
  return (
    <ToggleGroupPrimitive.Item
      className={cn(
        'text-muted-foreground hover:bg-accent hover:text-accent-foreground data-[state=on]:bg-primary data-[state=on]:text-primary-foreground inline-flex min-h-9 items-center justify-center rounded-lg px-3 text-sm font-semibold transition-colors focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
      ref={ref}
      data-slot="toggle-group-item"
      {...props}
    />
  )
})

ToggleGroup.displayName = ToggleGroupPrimitive.Root.displayName
ToggleGroupItem.displayName = ToggleGroupPrimitive.Item.displayName
