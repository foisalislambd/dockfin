import { useTheme } from 'next-themes'
import { useEffect, useState } from 'react'
import { appConfig } from '../config/app.config'

type BrandLogoProps = {
  /** Prefer mark (icon only) for small UI chrome; wordmark for auth/marketing. */
  variant?: 'mark' | 'wordmark'
  /** Override theme pairing (e.g. dark panel always uses dark logo). */
  forceTheme?: 'light' | 'dark'
  className?: string
  alt?: string
}

/** Dockfin brand image — swaps light/dark assets with the active theme. */
export function BrandLogo({
  variant = 'mark',
  forceTheme,
  className = 'h-8 w-8',
  alt,
}: BrandLogoProps) {
  const { resolvedTheme } = useTheme()
  const [mounted, setMounted] = useState(false)
  useEffect(() => setMounted(true), [])

  const theme = forceTheme ?? (mounted ? resolvedTheme : undefined)
  const dark = theme === 'dark'
  const { brand } = appConfig
  const src =
    variant === 'wordmark'
      ? dark
        ? brand.logo
        : brand.logoLight
      : dark
        ? brand.mark
        : brand.markLight

  return (
    <img
      src={src}
      alt={alt ?? brand.name}
      className={`object-contain ${className}`}
      draggable={false}
    />
  )
}
