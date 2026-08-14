import styles from './underline-tabs.module.scss'
import { cn } from '@/lib/utils'

export interface UnderlineTab<T extends string> {
  label: string
  value: T
}

export function UnderlineTabs<T extends string>({
  activeTab,
  className,
  idBase,
  label,
  onTabChange,
  tabs,
}: {
  activeTab: T
  className?: string
  idBase: string
  label: string
  onTabChange: (tab: T) => void
  tabs: readonly UnderlineTab<T>[]
}) {
  function focusTab(index: number) {
    const tab = tabs[index]
    if (!tab) return
    onTabChange(tab.value)
    requestAnimationFrame(() => {
      document.getElementById(`${idBase}-tab-${tab.value}`)?.focus()
    })
  }

  return (
    <div
      aria-label={label}
      className={cn(styles.tabs, className)}
      role="tablist"
    >
      {tabs.map((tab, index) => (
        <button
          aria-controls={`${idBase}-panel-${tab.value}`}
          aria-selected={activeTab === tab.value}
          className={styles.tab}
          id={`${idBase}-tab-${tab.value}`}
          key={tab.value}
          onClick={() => onTabChange(tab.value)}
          onKeyDown={(event) => {
            if (event.key === 'ArrowRight') {
              event.preventDefault()
              focusTab((index + 1) % tabs.length)
            } else if (event.key === 'ArrowLeft') {
              event.preventDefault()
              focusTab((index - 1 + tabs.length) % tabs.length)
            } else if (event.key === 'Home') {
              event.preventDefault()
              focusTab(0)
            } else if (event.key === 'End') {
              event.preventDefault()
              focusTab(tabs.length - 1)
            }
          }}
          role="tab"
          tabIndex={activeTab === tab.value ? 0 : -1}
          type="button"
        >
          {tab.label}
        </button>
      ))}
    </div>
  )
}
