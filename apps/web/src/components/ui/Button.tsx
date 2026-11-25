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
      className={`inline-flex h-10 items-center justify-center gap-2 rounded-lg px-4 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-50 sm:h-9 ${
        primary
          ? 'bg-brand-500 text-white shadow-[0_1px_2px_rgba(70,95,255,0.2)] hover:bg-brand-600'
          : 'border border-gray-200 bg-white text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-transparent dark:text-[#dce1e8] dark:hover:bg-white/5'
      }`}
    >
      {children}
    </button>
  )
}
