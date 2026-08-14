import type { CSSProperties } from 'react'
import { cn } from '@/lib/utils'
import styles from './skeleton.module.scss'

export function Skeleton({
  className,
  style,
}: {
  className?: string
  style?: CSSProperties
}) {
  return (
    <span
      aria-hidden="true"
      className={cn(styles.skeleton, className)}
      style={style}
    />
  )
}
