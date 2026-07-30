import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { CreatePageShell, FormActions, FormInput, FormSelect } from '../components/ui/forms'
import { api, fetchAllEnvironments, LAST_ENV_KEY } from '../lib/api'

export function CreateServicePage() {
  const nav = useNavigate()
  const qc = useQueryClient()
  const templates = useQuery({ queryKey: ['templates'], queryFn: api.templates })
  const dests = useQuery({ queryKey: ['destinations'], queryFn: api.destinations })
  const envs = useQuery({ queryKey: ['all-environments'], queryFn: fetchAllEnvironments })
  const [search, setSearch] = useState('')
  const [form, setForm] = useState({
    name: '',
    environment_id: '',
    destination_id: '',
    template: '',
  })

  useEffect(() => {
    const saved = localStorage.getItem(LAST_ENV_KEY) || ''
    const list = envs.data || []
    const pick = (saved && list.find((e) => e.id === saved)?.id) || list[0]?.id || ''
    const firstTpl = templates.data?.templates?.[0]?.type || ''
    setForm((f) => ({
      ...f,
      environment_id: f.environment_id || pick,
      destination_id: f.destination_id || dests.data?.destinations?.[0]?.id || '',
      template: f.template || firstTpl,
    }))
  }, [envs.data, dests.data, templates.data])

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    const list = templates.data?.templates || []
    if (!q) return list.slice(0, 48)
    return list
      .filter(
        (t) =>
          t.name.toLowerCase().includes(q) ||
          t.type.toLowerCase().includes(q) ||
          (t.description || '').toLowerCase().includes(q) ||
          (t.category || '').toLowerCase().includes(q),
      )
      .slice(0, 48)
  }, [templates.data, search])

  const selected = (templates.data?.templates || []).find((t) => t.type === form.template)

  const create = useMutation({
    mutationFn: () => api.createService(form),
    onSuccess: () => {
      if (form.environment_id) localStorage.setItem(LAST_ENV_KEY, form.environment_id)
      void qc.invalidateQueries({ queryKey: ['services'] })
      void nav({ to: '/services' })
    },
  })

  return (
    <CreatePageShell
      title="New service"
      subtitle="Pick a one-click template and deploy it to a destination."
      backTo="/services"
      backLabel="Back to services"
    >
      <form
        className="space-y-6"
        onSubmit={(e: FormEvent) => {
          e.preventDefault()
          create.mutate()
        }}
      >
        <section className="space-y-4">
          <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">Basics</h2>
          <FormInput
            label="Name"
            value={form.name}
            onChange={(v) => setForm({ ...form, name: v })}
            placeholder={selected?.name || 'my-service'}
          />
          <div className="grid gap-4 sm:grid-cols-2">
            <FormSelect label="Environment" value={form.environment_id} onChange={(v) => setForm({ ...form, environment_id: v })}>
              <option value="">Select…</option>
              {(envs.data || []).map((e) => (
                <option key={e.id} value={e.id}>
                  {e.project_name} / {e.name}
                </option>
              ))}
            </FormSelect>
            <FormSelect
              label="Destination"
              value={form.destination_id}
              onChange={(v) => setForm({ ...form, destination_id: v })}
              required={false}
              hint="optional"
            >
              <option value="">Optional…</option>
              {(dests.data?.destinations || []).map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name} ({d.network})
                </option>
              ))}
            </FormSelect>
          </div>
        </section>

        <section className="space-y-3">
          <div className="flex flex-wrap items-end justify-between gap-3">
            <h2 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">Template</h2>
            <input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search catalog…"
              className="h-8 w-full max-w-xs rounded-md border border-gray-200 bg-white px-3 text-sm dark:border-gray-700 dark:bg-gray-900"
            />
          </div>
          {selected && (
            <div className="rounded-xl border border-brand-200 bg-brand-50/70 px-4 py-3 text-sm dark:border-brand-500/30 dark:bg-brand-500/10">
              <span className="font-semibold text-brand-700 dark:text-brand-300">{selected.name}</span>
              <span className="text-gray-500 dark:text-gray-400"> — {selected.description}</span>
            </div>
          )}
          <div className="grid max-h-[28rem] gap-2 overflow-y-auto sm:grid-cols-2">
            {filtered.map((t) => {
              const active = form.template === t.type
              return (
                <button
                  key={t.type}
                  type="button"
                  onClick={() =>
                    setForm((f) => ({
                      ...f,
                      template: t.type,
                      name: f.name || t.name.toLowerCase().replace(/\s+/g, '-'),
                    }))
                  }
                  className={`rounded-lg border p-2.5 text-left transition ${
                    active
                      ? 'border-brand-500 bg-brand-50 ring-1 ring-brand-500/30 dark:bg-brand-500/10'
                      : 'border-gray-200 hover:border-gray-300 dark:border-gray-700'
                  }`}
                >
                  <div className="text-sm font-medium text-gray-900 dark:text-white">{t.name}</div>
                  <div className="mt-0.5 line-clamp-2 text-xs text-gray-500">{t.description}</div>
                  {t.category && (
                    <div className="mt-2 text-[10px] font-semibold tracking-wide text-gray-400 uppercase">
                      {t.category}
                    </div>
                  )}
                </button>
              )
            })}
          </div>
          {!filtered.length && (
            <p className="text-sm text-gray-500">No templates match your search.</p>
          )}
        </section>

        {create.error && (
          <p className="text-sm text-error-500" role="alert">
            {create.error.message}
          </p>
        )}
        <FormActions busy={create.isPending} submitLabel="Create service" cancelTo="/services" />
      </form>
    </CreatePageShell>
  )
}
