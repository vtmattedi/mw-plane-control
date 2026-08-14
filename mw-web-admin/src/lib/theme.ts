export type Theme = 'light' | 'dark'

const storageKey = 'mw-control-theme'

function storedTheme(): Theme | null {
  try {
    const value = window.localStorage.getItem(storageKey)

    return value === 'light' || value === 'dark' ? value : null
  } catch {
    return null
  }
}

export function preferredTheme(): Theme {
  return (
    storedTheme() ??
    (window.matchMedia('(prefers-color-scheme: dark)').matches
      ? 'dark'
      : 'light')
  )
}

export function applyTheme(theme: Theme, persist = false): void {
  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', theme === 'dark' ? '#101113' : '#24262b')

  if (!persist) return

  try {
    window.localStorage.setItem(storageKey, theme)
  } catch {
    // A preferência continua aplicada durante a sessão atual.
  }
}

export function initializeTheme(): void {
  applyTheme(preferredTheme())
}
