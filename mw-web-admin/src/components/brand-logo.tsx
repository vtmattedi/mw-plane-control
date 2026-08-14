import { Link } from 'react-router-dom'
import logo from '@/assets//branding/mattediworks/logo-right.svg'
import darkLogo from '@/assets/branding/mattediworks/logo-right-dark.svg'
import symbol from '@/assets/branding/mattediworks/logo-only.svg'
import darkSymbol from '@/assets/branding/mattediworks/logo-only-dark.svg'
import { cn } from '@/lib/utils'

interface BrandLogoProps {
  compact?: boolean
  className?: string
  linkTo?: string
}

export function BrandLogo({
  compact = false,
  className,
  linkTo,
}: BrandLogoProps) {
  const image = (
    <span
      aria-label="MattediWorks"
      className="relative block h-full w-full"
      role="img"
    >
      <img
        alt=""
        aria-hidden="true"
        className={cn(
          'brand-logo-image brand-logo-light h-full w-full object-contain',
          compact && 'brand-logo-symbol',
        )}
        src={compact ? symbol : logo}
      />
      <img
        alt=""
        aria-hidden="true"
        className={cn(
          'brand-logo-image brand-logo-dark h-full w-full object-contain',
          compact && 'brand-logo-symbol',
        )}
        src={compact ? darkSymbol : darkLogo}
      />
    </span>
  )

  if (!linkTo) return image

  return (
    <Link
      aria-label="MattediWorks — início"
      className={cn('brand-logo-link', className)}
      to={linkTo}
    >
      {image}
    </Link>
  )
}
