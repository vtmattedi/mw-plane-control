export type Locale = 'pt-BR' | 'en-US'

export function formatCount(value: number | undefined, locale: Locale) {
  if (value === undefined) return '—'
  return new Intl.NumberFormat(locale).format(Math.round(value))
}

export function formatBytes(value: number | undefined, locale: Locale) {
  if (value === undefined) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = Math.max(0, value)
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit++
  }
  const digits = unit === 0 || size >= 100 ? 0 : size >= 10 ? 1 : 2
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: digits }).format(size)} ${units[unit]}`
}

/** Messages or bytes per minute, as reported in the broker's load averages. */
export function formatRate(value: number | undefined, locale: Locale) {
  if (value === undefined) return '—'
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(
    value,
  )
}

export function formatDuration(seconds: number | undefined) {
  if (seconds === undefined || seconds < 0) return '—'
  const days = Math.floor(seconds / 86_400)
  const hours = Math.floor((seconds % 86_400) / 3_600)
  const minutes = Math.floor((seconds % 3_600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m`
  return `${Math.floor(seconds)}s`
}
