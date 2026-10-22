import { ChevronRight, Inbox } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { usePreferences } from '@/preferences/preferences-context'
import { StatusBadge } from './status-badge'

function display(value: unknown, language: 'pt' | 'en') {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'boolean')
    return value
      ? language === 'pt'
        ? 'Ativo'
        : 'Active'
      : language === 'pt'
        ? 'Inativo'
        : 'Inactive'
  if (Array.isArray(value)) return value.join(', ') || '—'
  if (typeof value === 'object') return JSON.stringify(value)
  const raw = String(value)
  if (/^\d{4}-\d{2}-\d{2}T/.test(raw)) {
    const date = new Date(raw)
    if (!Number.isNaN(date.getTime())) {
      return new Intl.DateTimeFormat(language === 'pt' ? 'pt-BR' : 'en-US', {
        dateStyle: 'medium',
        timeStyle: 'short',
      }).format(date)
    }
  }
  return raw.replaceAll('_', ' ')
}

function valueAtPath(row: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((value, key) => {
    if (!value || typeof value !== 'object') return undefined
    return (value as Record<string, unknown>)[key]
  }, row)
}

export type ResourceColumn = { key: string; label: string }

export function ResourceTable({
  rows,
  columns,
  emptyTitle,
  emptyDescription,
  renderActions,
  getRowHref,
}: {
  rows: Record<string, unknown>[]
  columns: ResourceColumn[]
  emptyTitle: string
  emptyDescription: string
  renderActions?: (row: Record<string, unknown>) => ReactNode
  getRowHref?: (row: Record<string, unknown>) => string | undefined
}) {
  const { language, tr } = usePreferences()
  if (!rows.length) {
    return (
      <div className="grid min-h-56 place-items-center p-8 text-center">
        <div>
          <div className="bg-muted mx-auto mb-4 grid size-11 place-items-center rounded-2xl">
            <Inbox aria-hidden="true" className="text-muted-foreground size-5" />
          </div>
          <h3 className="text-sm font-semibold">{emptyTitle}</h3>
          <p className="text-muted-foreground mx-auto mt-1 max-w-sm text-sm">
            {emptyDescription}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column.key}>{column.label}</th>
            ))}
            <th className={renderActions ? 'min-w-56' : 'w-12'}>
              <span className="sr-only">{tr('Ações')}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr className="group" key={String(row.id ?? index)}>
              {columns.map((column, columnIndex) => {
                const value = valueAtPath(row, column.key)
                const href = getRowHref?.(row)
                const isStatus = [
                  'enabled',
                  'status',
                  'delivery_status',
                  'severity',
                ].includes(column.key)
                const content = isStatus ? (
                  <StatusBadge
                    tone={
                      value === true ||
                      value === 'ok' ||
                      value === 'delivered' ||
                      value === 'success'
                        ? 'positive'
                        : value === false ||
                            value === 'failed' ||
                            value === 'error'
                          ? 'danger'
                          : 'neutral'
                    }
                  >
                    {display(value, language)}
                  </StatusBadge>
                ) : (
                  <span
                    className={
                      columnIndex === 0
                        ? 'font-medium'
                        : 'text-muted-foreground'
                    }
                  >
                    {display(value, language)}
                  </span>
                )
                return (
                  <td key={column.key}>
                    {href ? (
                      <Link
                        className={`block rounded-sm underline-offset-4 hover:underline ${columnIndex === 0 ? 'text-primary font-semibold' : ''}`}
                        to={href}
                      >
                        {content}
                      </Link>
                    ) : (
                      content
                    )}
                  </td>
                )
              })}
              <td>
                {renderActions ? (
                  renderActions(row)
                ) : getRowHref?.(row) ? (
                  <Link
                    aria-label={`${tr('Abrir')} ${display(valueAtPath(row, columns[0].key), language)}`}
                    className="text-muted-foreground hover:text-primary inline-flex rounded-md p-1 opacity-60 transition-opacity group-hover:opacity-100"
                    to={getRowHref(row)!}
                  >
                    <ChevronRight className="size-4" />
                  </Link>
                ) : (
                  <ChevronRight className="text-muted-foreground size-4 opacity-0 transition-opacity group-hover:opacity-100" />
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
