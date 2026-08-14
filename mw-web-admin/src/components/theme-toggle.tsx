import { Moon, Sun } from 'lucide-react'
import { useState } from 'react'
import { applyTheme, preferredTheme, type Theme } from '@/lib/theme'
import { cn } from '@/lib/utils'
import styles from './theme-toggle.module.scss'

export function ThemeToggle({ className }: { className?: string }) {
  const [theme, setTheme] = useState<Theme>(() => preferredTheme())

  function toggle() {
    const nextTheme = theme === 'light' ? 'dark' : 'light'
    applyTheme(nextTheme, true)
    setTheme(nextTheme)
  }

  return (
    <button
      aria-label={
        theme === 'light' ? 'Ativar modo escuro' : 'Ativar modo claro'
      }
      className={cn(styles.toggle, className)}
      type="button"
      onClick={toggle}
    >
      {theme === 'light' ? (
        <Moon aria-hidden="true" />
      ) : (
        <Sun aria-hidden="true" />
      )}
    </button>
  )
}
