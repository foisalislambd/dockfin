import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Input } from './Input'
import { InfoHint } from './forms'
import { Modal } from './Modal'
import { PanelSkeleton } from './Skeleton'
import { Btn } from './Button'

describe('Input', () => {
  it('reports the typed value and blur', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    const onBlur = vi.fn()
    render(<Input label="Name" value="" onChange={onChange} onBlur={onBlur} />)
    await user.type(screen.getByRole('textbox', { name: 'Name' }), 'web')
    expect(onChange).toHaveBeenCalled()
    await user.tab()
    expect(onBlur).toHaveBeenCalled()
  })
})

describe('InfoHint', () => {
  it('opens on click and closes on Escape', async () => {
    const user = userEvent.setup()
    render(<InfoHint text="Used as the public hostname." />)
    const btn = screen.getByRole('button', { name: 'More info' })
    expect(btn).toHaveAttribute('aria-expanded', 'false')
    await user.click(btn)
    expect(btn).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('Used as the public hostname.')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(btn).toHaveAttribute('aria-expanded', 'false')
  })
})

describe('Modal', () => {
  it('focuses the first control and closes on Escape', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <Modal title="New project" onClose={onClose}>
        <label>
          Name
          <input name="name" />
        </label>
        <button type="button">Save</button>
      </Modal>,
    )
    expect(screen.getByRole('dialog', { name: 'New project' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus()
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalled()
  })
})

describe('loading and buttons', () => {
  it('exposes a status region and disables a pending button', () => {
    render(
      <div>
        <PanelSkeleton rows={2} />
        <Btn disabled>Saving…</Btn>
      </div>,
    )
    expect(screen.getByRole('status', { name: 'Loading' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled()
  })
})
