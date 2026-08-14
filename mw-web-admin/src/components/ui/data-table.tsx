import type { ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import type { PageMeta } from '@/lib/api'
import { Button } from './button'
import styles from './data-table.module.scss'

export interface DataTableColumn<T> {
  id: string
  header: ReactNode
  label?: string
  render: (row: T) => ReactNode
}

function columnLabel<T>(column: DataTableColumn<T>) {
  return (
    column.label ??
    (typeof column.header === 'string' ? column.header : column.id)
  )
}

export function DataTable<T>({
  caption,
  columns,
  getRowHref,
  getRowKey,
  rows,
}: {
  caption: string
  columns: DataTableColumn<T>[]
  getRowHref?: (row: T) => string
  getRowKey: (row: T) => string
  rows: T[]
}) {
  const navigate = useNavigate()

  function openRow(row: T) {
    if (getRowHref) navigate(getRowHref(row))
  }

  return (
    <div className={styles.wrapper}>
      <table className={styles.table}>
        <caption className="sr-only">{caption}</caption>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column.id} scope="col">
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const clickable = getRowHref !== undefined
            return (
              <tr
                aria-label={clickable ? `Abrir ${getRowKey(row)}` : undefined}
                data-clickable={clickable || undefined}
                key={getRowKey(row)}
                tabIndex={clickable ? 0 : undefined}
                onClick={(event) => {
                  if (
                    event.target instanceof HTMLElement &&
                    event.target.closest(
                      'a, button, input, select, textarea, [role="button"], [role="switch"]',
                    )
                  ) {
                    return
                  }
                  openRow(row)
                }}
                onKeyDown={(event) => {
                  if (!clickable) return
                  if (event.key !== 'Enter' && event.key !== ' ') return
                  if (
                    event.target instanceof HTMLElement &&
                    event.target.closest(
                      'a, button, input, select, textarea, [role="button"], [role="switch"]',
                    )
                  ) {
                    return
                  }
                  event.preventDefault()
                  openRow(row)
                }}
              >
                {columns.map((column) => (
                  <td data-label={columnLabel(column)} key={column.id}>
                    {column.render(row)}
                  </td>
                ))}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

export function DataTablePagination({
  disabled = false,
  label,
  meta,
  onPageChange,
}: {
  disabled?: boolean
  label: string
  meta: PageMeta
  onPageChange: (page: number) => void
}) {
  if (meta.total === 0) return null

  const firstItem = (meta.currentPage - 1) * meta.perPage + 1
  const lastItem = Math.min(meta.currentPage * meta.perPage, meta.total)

  return (
    <nav aria-label={label} className={styles.pagination}>
      <p aria-live="polite">
        {firstItem}–{lastItem} de {meta.total}
        <span>
          Página {meta.currentPage} de {meta.lastPage}
        </span>
      </p>
      <div>
        <Button
          disabled={disabled || meta.currentPage <= 1}
          variant="secondary"
          onClick={() => onPageChange(meta.currentPage - 1)}
        >
          Anterior
        </Button>
        <Button
          disabled={disabled || meta.currentPage >= meta.lastPage}
          variant="secondary"
          onClick={() => onPageChange(meta.currentPage + 1)}
        >
          Próxima
        </Button>
      </div>
    </nav>
  )
}
