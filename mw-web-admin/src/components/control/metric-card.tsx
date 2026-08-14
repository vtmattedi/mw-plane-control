import type { LucideIcon } from 'lucide-react'
import { Card } from '@/components/ui/card'

export function MetricCard({
  label,
  value,
  note,
  icon: Icon,
  tone = 'default',
}: {
  label: string
  value: string | number
  note: string
  icon: LucideIcon
  tone?: 'default' | 'accent'
}) {
  return (
    <Card
      className={
        tone === 'accent'
          ? 'border-primary/15 bg-primary text-primary-foreground shadow-primary/10 shadow-xl'
          : 'border-border/70 shadow-sm'
      }
    >
      <div className="p-5" data-slot="metric-card-content">
        <div className="flex items-center justify-between">
          <p
            className={
              tone === 'accent'
                ? 'text-primary-foreground/70 text-sm'
                : 'text-muted-foreground text-sm'
            }
          >
            {label}
          </p>
          <span
            className={
              tone === 'accent'
                ? 'grid size-9 place-items-center rounded-xl bg-white/10'
                : 'bg-accent text-accent-foreground grid size-9 place-items-center rounded-xl'
            }
          >
            <Icon className="size-4" aria-hidden="true" />
          </span>
        </div>
        <p className="mt-5 text-3xl font-semibold tracking-tight">{value}</p>
        <p
          className={
            tone === 'accent'
              ? 'text-primary-foreground/65 mt-1 text-xs'
              : 'text-muted-foreground mt-1 text-xs'
          }
        >
          {note}
        </p>
      </div>
    </Card>
  )
}
