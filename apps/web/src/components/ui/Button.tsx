import type { ReactNode } from 'react'

export function Btn({
  children,
  onClick,
  primary,
  type = 'button',
  disabled,
}: {
  children: ReactNode
  onClick?: () => void
  primary?: boolean
  type?: 'button' | 'submit'
  disabled?: boolean
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={`inline-flex h-8 items-center rounded-md px-2.5 text-xs font-medium transition disabled:cursor-not-allowed disabled:opacity-50 ${
        primary
          ? 'bg-brand-500 text-white hover:bg-brand-600'
          : 'border border-gray-200 text-gray-600 hover:bg-gray-50 dark:border-gray-800 dark:text-gray-300 dark:hover:bg-white/5'
      }`}
    >
      {children}
    </button>
  )
}
