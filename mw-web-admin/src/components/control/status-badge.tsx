import { cn } from '@/lib/utils'
import type { ReactNode } from 'react'

export function StatusBadge({
  children,
  tone = 'neutral',
}: {
  children: ReactNode
  tone?: 'positive' | 'warning' | 'danger' | 'info' | 'neutral'
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-semibold',
        tone === 'positive' && 'border-success/20 bg-success/10 text-success',
        tone === 'warning' && 'border-warning/20 bg-warning/10 text-warning',
        tone === 'danger' &&
          'border-destructive/20 bg-destructive/10 text-destructive',
        tone === 'info' &&
          'border-information/20 bg-information/10 text-information',
        tone === 'neutral' && 'border-border bg-muted text-muted-foreground',
      )}
    >
      <span className="size-1.5 rounded-full bg-current" />
      {children}
    </span>
  )
}
