import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LoginPage } from './Auth'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to }: { children: ReactNode; to: string }) => <a href={to}>{children}</a>,
  Navigate: () => null,
  useNavigate: () => () => undefined,
}))

vi.mock('../lib/auth', () => ({
  useAuth: () => ({ user: null, loading: false, refresh: async () => undefined }),
}))

vi.mock('../lib/webauthn', () => ({
  getPasskey: () => null,
}))

describe('LoginPage', () => {
  it('shows an inline error for a malformed email and keeps the password field', async () => {
    const user = userEvent.setup()
    render(<LoginPage />)
    const email = screen.getByLabelText('Email')
    await user.type(email, 'not-an-email')
    await user.tab()
    expect(screen.getByRole('alert')).toHaveTextContent('Enter a valid email address.')
    expect(email).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByPlaceholderText('Enter your password')).toBeInTheDocument()
  })
})
