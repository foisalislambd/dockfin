import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { PendingDeletions } from './PendingDeletions'

const retryDeletion = vi.fn(async () => ({}))
const forgetDeletion = vi.fn(async () => ({}))

vi.mock('../lib/api', () => ({
  api: {
    deletions: async () => ({
      deletions: [
        {
          resource_type: 'application',
          resource_id: 'app-1',
          name: 'web',
          status: 'failed',
          error: 'ssh failed',
        },
      ],
    }),
    retryDeletion: (...args: unknown[]) => retryDeletion(...args),
    forgetDeletion: (...args: unknown[]) => forgetDeletion(...args),
  },
}))

describe('PendingDeletions', () => {
  it('retries a failed cleanup and forgets the record after confirmation', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <PendingDeletions />
      </QueryClientProvider>,
    )
    expect(await screen.findByText('web')).toBeInTheDocument()
    expect(screen.getByText('ssh failed')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Retry cleanup' }))
    expect(retryDeletion).toHaveBeenCalledWith('application', 'app-1')
    await user.click(screen.getByRole('button', { name: 'Forget anyway' }))
    expect(forgetDeletion).toHaveBeenCalledWith('application', 'app-1')
  })
})
