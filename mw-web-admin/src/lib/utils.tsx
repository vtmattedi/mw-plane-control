import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Dangerously set inner HTML for <b> tags in text
export function toHtml(text: string) {
  const parts = text.split(/(<b>.*?<\/b>)/g)
  return parts.map((part, index) => {
    if (part.startsWith('<b>') && part.endsWith('</b>')) {
      const content = part.slice(3, -4)
      return (
        <strong key={index} dangerouslySetInnerHTML={{ __html: content }} />
      )
    }
    return <span key={index}>{part}</span>
  })
}
