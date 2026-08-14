import {
  type ReactNode,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from 'react'
import { createPortal } from 'react-dom'
import styles from './delayed-tooltip.module.scss'

const VIEWPORT_GAP = 8
const TRIGGER_GAP = 7

interface TooltipPosition {
  left: number
  placement: 'bottom' | 'top'
  top: number
}

export function DelayedTooltip({
  children,
  delayMs = 650,
}: {
  children: ReactNode
  delayMs?: number
}) {
  const id = useId()
  const markerRef = useRef<HTMLSpanElement>(null)
  const tooltipRef = useRef<HTMLSpanElement>(null)
  const triggerRef = useRef<HTMLElement | null>(null)
  const timerRef = useRef<number | null>(null)
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState<TooltipPosition>({
    left: 0,
    placement: 'top',
    top: 0,
  })

  const clearTimer = () => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }

  const updatePosition = () => {
    const trigger = triggerRef.current
    const tooltip = tooltipRef.current
    if (!trigger || !tooltip) return

    const triggerRect = trigger.getBoundingClientRect()
    const tooltipRect = tooltip.getBoundingClientRect()
    const halfWidth = tooltipRect.width / 2
    const left = Math.min(
      window.innerWidth - VIEWPORT_GAP - halfWidth,
      Math.max(
        VIEWPORT_GAP + halfWidth,
        triggerRect.left + triggerRect.width / 2,
      ),
    )
    const placement =
      triggerRect.top >= tooltipRect.height + TRIGGER_GAP + VIEWPORT_GAP
        ? 'top'
        : 'bottom'

    setPosition({
      left,
      placement,
      top:
        placement === 'top'
          ? triggerRect.top - TRIGGER_GAP
          : triggerRect.bottom + TRIGGER_GAP,
    })
  }

  useEffect(() => {
    const trigger = markerRef.current?.parentElement
    if (!trigger) return

    triggerRef.current = trigger
    const describedBy = trigger.getAttribute('aria-describedby')
    trigger.setAttribute(
      'aria-describedby',
      describedBy ? `${describedBy} ${id}` : id,
    )

    const show = () => {
      clearTimer()
      timerRef.current = window.setTimeout(() => {
        setOpen(true)
        timerRef.current = null
      }, delayMs)
    }
    const hide = () => {
      clearTimer()
      setOpen(false)
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') hide()
    }

    trigger.addEventListener('pointerenter', show)
    trigger.addEventListener('pointerleave', hide)
    trigger.addEventListener('focusin', show)
    trigger.addEventListener('focusout', hide)
    trigger.addEventListener('keydown', handleKeyDown)

    return () => {
      clearTimer()
      trigger.removeEventListener('pointerenter', show)
      trigger.removeEventListener('pointerleave', hide)
      trigger.removeEventListener('focusin', show)
      trigger.removeEventListener('focusout', hide)
      trigger.removeEventListener('keydown', handleKeyDown)
      const currentDescribedBy = trigger
        .getAttribute('aria-describedby')
        ?.split(/\s+/)
        .filter((value) => value && value !== id)
        .join(' ')
      if (currentDescribedBy) {
        trigger.setAttribute('aria-describedby', currentDescribedBy)
      } else {
        trigger.removeAttribute('aria-describedby')
      }
      triggerRef.current = null
    }
  }, [delayMs, id])

  useLayoutEffect(() => {
    if (open) updatePosition()
  }, [open])

  useEffect(() => {
    if (!open) return

    const reposition = () => updatePosition()
    window.addEventListener('resize', reposition)
    window.addEventListener('scroll', reposition, true)
    return () => {
      window.removeEventListener('resize', reposition)
      window.removeEventListener('scroll', reposition, true)
    }
  }, [open])

  return (
    <>
      <span aria-hidden="true" className={styles.marker} ref={markerRef} />
      {typeof document !== 'undefined' &&
        createPortal(
          <span
            className={styles.tooltip}
            data-open={open || undefined}
            data-placement={position.placement}
            id={id}
            ref={tooltipRef}
            role="tooltip"
            style={{ left: position.left, top: position.top }}
          >
            {children}
          </span>,
          document.body,
        )}
    </>
  )
}
