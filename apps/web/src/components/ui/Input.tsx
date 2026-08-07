export function Input({
  label,
  value,
  onChange,
  onBlur,
  required = true,
}: {
  label: string
  value: string
  onChange: (v: string) => void
  onBlur?: () => void
  required?: boolean
}) {
  return (
    <label className="block text-sm">
      <span className="mb-1 block text-gray-500 dark:text-gray-400">{label}</span>
      <input
        required={required}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onBlur={() => onBlur?.()}
        className="panel-field w-full rounded-lg px-3 py-2 outline-none focus:border-brand-400 focus:ring-2 focus:ring-brand-500/20"
      />
    </label>
  )
}
