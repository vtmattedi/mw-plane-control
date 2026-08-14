import { useSearchParams } from 'react-router-dom'

export function readUrlParam(
  params: URLSearchParams,
  key: string,
  fallback = '',
) {
  return params.get(key) ?? fallback
}

export function readUrlPage(params: URLSearchParams) {
  const page = Number(params.get('page') ?? '1')
  return Number.isInteger(page) && page > 0 ? page : 1
}

export function useUrlFilters() {
  const [searchParams, setSearchParams] = useSearchParams()

  function setUrlFilter(key: string, value: string, resetPage = true) {
    const next = new URLSearchParams(searchParams)
    if (value) {
      next.set(key, value)
    } else {
      next.delete(key)
    }
    if (resetPage) next.delete('page')
    setSearchParams(next, { replace: true })
  }

  function setUrlPage(page: number) {
    const next = new URLSearchParams(searchParams)
    if (page <= 1) {
      next.delete('page')
    } else {
      next.set('page', String(page))
    }
    setSearchParams(next, { replace: true })
  }

  return { searchParams, setUrlFilter, setUrlPage }
}
