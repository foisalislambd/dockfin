import { appConfig } from '../config/app.config'

type BrandLogoProps = {
  /** Prefer mark (icon only) for small UI chrome; wordmark for auth/marketing. */
  variant?: 'mark' | 'wordmark'
  className?: string
  alt?: string
}

/** Dockfin brand image — mark for nav/favicon-sized spots, wordmark for login hero. */
export function BrandLogo({ variant = 'mark', className = 'h-8 w-8', alt }: BrandLogoProps) {
  const src = variant === 'wordmark' ? appConfig.brand.logo : appConfig.brand.mark
  return (
    <img
      src={src}
      alt={alt ?? appConfig.brand.name}
      className={`object-contain ${className}`}
      draggable={false}
    />
  )
}
