import type { LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'

export function PageHeader({
  eyebrow,
  title,
  description,
  icon: Icon,
  action,
  onAction,
}: {
  eyebrow: string
  title: string
  description: string
  icon: LucideIcon
  action?: string
  onAction?: () => void
}) {
  return (
    <header className="border-border/70 flex flex-col gap-5 border-b pb-7 sm:flex-row sm:items-end sm:justify-between">
      <div className="flex min-w-0 gap-4">
        <div className="bg-primary/10 text-primary hidden size-12 shrink-0 place-items-center rounded-2xl sm:grid">
          <Icon className="size-5" aria-hidden="true" />
        </div>
        <div>
          <p className="text-primary mb-1 text-xs font-bold tracking-[0.16em] uppercase">
            {eyebrow}
          </p>
          <h1 className="font-display text-foreground text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
            {title}
          </h1>
          <p className="text-muted-foreground mt-2 max-w-2xl text-sm leading-6">
            {description}
          </p>
        </div>
      </div>
      {action && (
        <Button className="shrink-0" onClick={onAction}>
          {action}
        </Button>
      )}
    </header>
  )
}
