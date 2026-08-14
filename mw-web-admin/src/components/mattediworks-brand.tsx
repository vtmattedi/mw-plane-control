import acrossDarkLogo from '@/assets/branding/mattediworks/logo-across-dark.svg'
import acrossLogo from '@/assets/branding/mattediworks/logo-across.svg'
import belowDarkLogo from '@/assets/branding/mattediworks/logo-below-dark.svg'
import belowLogo from '@/assets/branding/mattediworks/logo-below.svg'
import onlyDarkLogo from '@/assets/branding/mattediworks/logo-only-dark.svg'
import onlyLogo from '@/assets/branding/mattediworks/logo-only.svg'
import rightDarkLogo from '@/assets/branding/mattediworks/logo-right-dark.svg'
import rightLogo from '@/assets/branding/mattediworks/logo-right.svg'
import { cn } from '@/lib/utils'
import styles from './mattediworks-brand.module.scss'

export type MattediWorksBrandVariant = 'only' | 'across' | 'below' | 'right'

interface MattediWorksBrandProps {
  className?: string
  variant?: MattediWorksBrandVariant
  style?: React.CSSProperties
  imgClassName?: string
}

const logos: Record<MattediWorksBrandVariant, { dark: string; light: string }> =
  {
    only: { dark: onlyDarkLogo, light: onlyLogo },
    across: { dark: acrossDarkLogo, light: acrossLogo },
    below: { dark: belowDarkLogo, light: belowLogo },
    right: { dark: rightDarkLogo, light: rightLogo },
  }

export function MattediWorksBrand({
  className,
  style,
  imgClassName,
  variant = 'right',
}: MattediWorksBrandProps) {
  const selected = logos[variant]

  return (
    <span
      aria-label="MattediWorks"
      className={cn(styles.brand, className)}
      data-variant={variant}
      style={style}
      role="img"
    >
      <img
        aria-hidden="true"
        alt=""
        className={cn(styles.image, styles.light, imgClassName)}
        src={selected.light}
      />
      <img
        aria-hidden="true"
        alt=""
        className={cn(styles.image, styles.dark, imgClassName)}
        src={selected.dark}
      />
    </span>
  )
}
