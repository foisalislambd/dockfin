import { useState, type FormEvent } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Btn } from './Button'
import { Header } from './Header'
import { Input } from './Input'
import { ConfirmProvider, useConfirm } from '../ConfirmDialog'

function ControlledName({ required = true }: { required?: boolean }) {
  const [value, setValue] = useState('')
  return <Input label="Name" value={value} onChange={setValue} required={required} />
}

function ConfirmProbe({
  title,
  message,
  confirmLabel,
  danger,
}: {
  title?: string
  message: string
  confirmLabel?: string
  danger?: boolean
}) {
  const confirm = useConfirm()
  const [result, setResult] = useState('idle')
  return (
    <>
      <button
        type="button"
        onClick={() => {
          void confirm({ title, message, confirmLabel, danger }).then((ok) => setResult(ok ? 'yes' : 'no'))
        }}
      >
        Ask
      </button>
      <p role="status">{result}</p>
    </>
  )
}

describe('Btn', () => {
  it('runs the click handler', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(
      <Btn primary onClick={onClick}>
        Save
      </Btn>,
    )
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('does not activate a disabled button', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(
      <Btn disabled onClick={onClick}>
        Saving…
      </Btn>,
    )
    const button = screen.getByRole('button', { name: 'Saving…' })
    expect(button).toBeDisabled()
    await user.click(button)
    expect(onClick).not.toHaveBeenCalled()
  })

  it('submits the parent form and ignores an empty label as a second control', async () => {
    const user = userEvent.setup()
    const onSubmit = vi.fn((event: FormEvent) => event.preventDefault())
    render(
      <form onSubmit={onSubmit}>
        <Btn type="submit">Create</Btn>
        <Btn>{''}</Btn>
      </form>,
    )
    await user.click(screen.getByRole('button', { name: 'Create' }))
    expect(onSubmit).toHaveBeenCalledOnce()
    expect(screen.getAllByRole('button')).toHaveLength(2)
  })
})

describe('Input', () => {
  it('shows the typed value in the named field', async () => {
    const user = userEvent.setup()
    render(<ControlledName />)
    const field = screen.getByRole('textbox', { name: 'Name' })
    expect(field).toBeRequired()
    await user.type(field, 'api')
    expect(field).toHaveValue('api')
  })

  it('allows an empty optional field', async () => {
    const user = userEvent.setup()
    const onBlur = vi.fn()
    render(<Input label="Description" value="" onChange={() => undefined} onBlur={onBlur} required={false} />)
    const field = screen.getByRole('textbox', { name: 'Description' })
    expect(field).not.toBeRequired()
    expect(field).toHaveValue('')
    await user.click(field)
    await user.tab()
    expect(onBlur).toHaveBeenCalledOnce()
  })

  it('keeps markup-like input as a value', async () => {
    const user = userEvent.setup()
    render(<ControlledName />)
    const field = screen.getByRole('textbox', { name: 'Name' })
    const payload = '"><img src=x onerror=alert(1)>'
    await user.type(field, payload)
    expect(field).toHaveValue(payload)
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })
})

describe('Header', () => {
  it('renders the title, subtitle, and actions', () => {
    render(<Header title="Projects" subtitle="Everything in this team" actions={<button type="button">New project</button>} />)
    expect(screen.getByRole('heading', { level: 1, name: 'Projects' })).toBeInTheDocument()
    expect(screen.getByText('Everything in this team')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'New project' })).toBeInTheDocument()
  })

  it('omits the subtitle and actions when they are absent', () => {
    render(<Header title="Servers" />)
    expect(screen.getByRole('heading', { name: 'Servers' })).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByText('Everything in this team')).not.toBeInTheDocument()
  })

  it('still exposes a heading when the title is blank and keeps markup as text', () => {
    const { unmount } = render(<Header title="" />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('')
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    unmount()

    const payload = '<script>alert(1)</script>'
    render(<Header title="Projects" subtitle={payload} />)
    expect(screen.getByRole('heading', { name: 'Projects' })).toBeInTheDocument()
    expect(screen.getByText(payload)).toBeInTheDocument()
  })
})

describe('ConfirmProvider', () => {
  it('resolves when the confirm action is chosen', async () => {
    const user = userEvent.setup()
    render(
      <ConfirmProvider>
        <ConfirmProbe title="Delete project" message="This cannot be undone." confirmLabel="Delete" danger />
      </ConfirmProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    expect(screen.getByRole('dialog', { name: 'Delete project' })).toBeInTheDocument()
    expect(screen.getByText('This cannot be undone.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Delete' }))
    expect(screen.getByRole('status')).toHaveTextContent('yes')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('resolves false from Cancel and from Escape', async () => {
    const user = userEvent.setup()
    const { unmount } = render(
      <ConfirmProvider>
        <ConfirmProbe message="" />
      </ConfirmProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    expect(screen.getByRole('dialog', { name: 'Confirm' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('status')).toHaveTextContent('no')

    unmount()
    render(
      <ConfirmProvider>
        <ConfirmProbe message="Leave this page?" />
      </ConfirmProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    await user.keyboard('{Escape}')
    expect(screen.getByRole('status')).toHaveTextContent('no')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders an abusive message as text and falls back to window.confirm', async () => {
    const user = userEvent.setup()
    const payload = '<img src=x onerror=alert(1)>'
    const { unmount } = render(
      <ConfirmProvider>
        <ConfirmProbe message={payload} />
      </ConfirmProvider>,
    )
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    expect(screen.getByText(payload)).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
    unmount()

    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    render(<ConfirmProbe message="Leave without saving?" />)
    await user.click(screen.getByRole('button', { name: 'Ask' }))
    expect(confirm).toHaveBeenCalledWith('Leave without saving?')
    expect(screen.getByRole('status')).toHaveTextContent('no')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    confirm.mockRestore()
  })
})
