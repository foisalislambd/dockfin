import { useState } from 'react'
import { cn } from '../lib/cn'

type Props = {
  src?: string | null
  name: string
  className?: string
  imgClassName?: string
}

/** Service catalog / list logo with letter fallback when image is missing. */
export function ServiceLogo({ src, name, className, imgClassName }: Props) {
  const [failed, setFailed] = useState(false)
  const letter = (name || '?').trim().charAt(0).toUpperCase() || '?'

  if (!src || failed) {
    return (
      <span
        className={cn(
          'inline-flex shrink-0 items-center justify-center rounded-lg bg-brand-50 text-sm font-semibold text-brand-600 dark:bg-brand-500/15 dark:text-brand-300',
          className,
        )}
        aria-hidden
      >
        {letter}
      </span>
    )
  }

  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center justify-center overflow-hidden rounded-lg bg-gray-50 dark:bg-white/10',
        className,
      )}
    >
      <img
        src={src}
        alt=""
        loading="lazy"
        decoding="async"
        className={cn('h-full w-full object-contain p-1', imgClassName)}
        onError={() => setFailed(true)}
      />
    </span>
  )
}
