import { CircleCheck, LoaderCircle } from 'lucide-react'
import { lazy, Suspense, useEffect, useState } from 'react'

const AnimatedCircleCheckIcon = lazy(async () => {
  const icons = await import('lucide-animated')
  return { default: icons.CircleCheckIcon }
})

interface AnimatedStatusIconProps {
  active: boolean
  size?: number
}

function useReducedMotion() {
  const [reduced, setReduced] = useState(
    () =>
      window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? true,
  )

  useEffect(() => {
    const media = window.matchMedia?.('(prefers-reduced-motion: reduce)')
    if (!media) return
    const update = () => setReduced(media.matches)
    media.addEventListener('change', update)
    return () => media.removeEventListener('change', update)
  }, [])

  return reduced
}

export function AnimatedStatusIcon({
  active,
  size = 24,
}: AnimatedStatusIconProps) {
  const reducedMotion = useReducedMotion()

  if (!active) {
    return <LoaderCircle size={size} aria-hidden="true" />
  }
  if (reducedMotion) {
    return <CircleCheck size={size} aria-hidden="true" />
  }

  return (
    <Suspense fallback={<CircleCheck size={size} aria-hidden="true" />}>
      <AnimatedCircleCheckIcon
        size={size}
        animateOnHover
        aria-hidden="true"
        data-testid="animated-status-icon"
      />
    </Suspense>
  )
}
