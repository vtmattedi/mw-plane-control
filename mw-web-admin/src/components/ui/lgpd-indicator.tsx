import { Link } from 'react-router-dom'
import { ShieldCheckIcon } from '@/components/animated-icons/shield-check'
import { DelayedTooltip } from './delayed-tooltip'
import styles from './lgpd-indicator.module.scss'

type LgpdIndicatorProps = {
  noLink?: boolean
}

export function LgpdIndicator({ noLink = false }: LgpdIndicatorProps) {
  const content = (
    <>
      <ShieldCheckIcon aria-hidden="true" size={21} className={styles.icon} />
      <DelayedTooltip delayMs={250}>
        Estes dados são protegidos conforme a LGPD. Veja os detalhes em
        Integrações › LGPD.
      </DelayedTooltip>
    </>
  )

  if (noLink) {
    return (
      <span
        aria-label="Dados protegidos pela LGPD. Ver detalhes"
        className={styles.trigger}
      >
        {content}
      </span>
    )
  }

  return (
    <Link
      aria-label="Dados protegidos pela LGPD. Ver detalhes"
      className={styles.trigger}
      to="/integracoes?tab=lgpd"
    >
      {content}
    </Link>
  )
}
