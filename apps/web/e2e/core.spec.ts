import AxeBuilder from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

const pages = [
  '/dashboard',
  '/projects',
  '/servers',
  '/fleet',
  '/vulnerabilities',
  '/notifications',
  '/settings',
  '/storages',
  '/team',
  '/security',
  '/git-sources',
  '/terminal',
  '/audit',
  '/shared-variables',
]

async function axe(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).disableRules(['color-contrast']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(serious, JSON.stringify(serious.map((v) => ({ id: v.id, nodes: v.nodes.slice(0, 2) })), null, 2)).toEqual([])
}

async function signIn(page: Page) {
  const email = 'e2e@example.com'
  const password = 'password12345'
  await page.goto('/register')
  if (await page.getByRole('heading', { name: 'Create account' }).isVisible().catch(() => false)) {
    const registered = page.waitForResponse((r) => r.url().includes('/api/v1/auth/register') && r.request().method() === 'POST')
    await page.getByLabel('Name').fill('E2E')
    await page.getByLabel('Email').fill(email)
    await page.getByLabel('Password', { exact: true }).fill(password)
    await page.getByRole('button', { name: /create account|register|sign up/i }).click()
    const res = await registered
    if (res.ok()) {
      await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 })
      const body = (await res.json()) as { token?: string }
      return { token: body.token || '', password }
    }
  }
  await page.goto('/login')
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password', { exact: true }).fill(password)
  const loggedIn = page.waitForResponse((r) => r.url().includes('/api/v1/auth/login') && r.request().method() === 'POST')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  const res = await loggedIn
  const body = (await res.json()) as { token?: string }
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 20_000 })
  return { token: body.token || '', password }
}

test('core flows stay usable and accessible', async ({ page }, testInfo) => {
  await page.goto('/login')
  await axe(page)
  await page.getByLabel('Email').fill('not-an-email')
  await page.getByLabel('Email').blur()
  await expect(page.getByRole('alert')).toContainText('valid email')

  const session = await signIn(page)
  await axe(page)

  for (const path of pages) {
    await page.goto(path)
    await expect(page.getByRole('heading').first()).toBeVisible()
    await axe(page)
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow, path).toBeLessThanOrEqual(2)
  }

  await page.goto('/projects')
  await page.getByRole('button', { name: 'New project' }).first().click()
  const dialog = page.getByRole('dialog', { name: 'New project' })
  await expect(dialog.getByRole('button', { name: 'Create' })).toBeDisabled()
  const projectName = `e2e ${testInfo.project.name}`
  await dialog.getByLabel('Name').fill(projectName)
  await dialog.getByRole('button', { name: 'Create' }).click()
  const projectLink = page.getByRole('link', { name: new RegExp(projectName) }).first()
  await expect(projectLink).toBeVisible()
  if (!/\/environments\//.test(page.url())) {
    if (await page.getByRole('dialog', { name: 'New project' }).isVisible()) {
      await page.getByRole('button', { name: 'Close' }).click()
    }
    await projectLink.click()
  }
  await expect(page).toHaveURL(/\/projects\/.+\/environments\//)
  const ids = page.url().match(/\/projects\/([^/]+)\/environments\/([^/?]+)/)
  expect(ids).toBeTruthy()
  const created = await page.request.post('/api/v1/applications', {
    headers: { Authorization: `Bearer ${session.token}`, 'Content-Type': 'application/json' },
    data: { environment_id: ids![2], name: 'web', build_pack: 'dockerfile', dockerfile: 'FROM nginx:alpine\n' },
  })
  expect(created.ok(), await created.text()).toBeTruthy()
  const app = (await created.json()) as { id?: string; application?: { id: string } }
  const appID = app.id || app.application?.id
  expect(appID).toBeTruthy()
  await page.goto(`/applications/${appID}`)
  await expect(page.getByRole('heading').first()).toBeVisible()

  await page.getByRole('button', { name: 'Deploy' }).first().click()
  await expect(page.getByRole('alert').or(page.getByText(/destination|server|deploy/i)).first()).toBeVisible()

  for (const section of ['Domains', 'Environment Variables', 'Deployment Logs', 'Runtime Logs', 'Backups', 'Danger Zone']) {
    await page.getByRole('button', { name: section, exact: true }).click()
    await expect(page).toHaveURL(new RegExp(`/applications/${appID}`))
    await expect(page.locator('main')).toBeVisible()
  }
  await axe(page)

  const database = await page.request.post('/api/v1/databases', {
    headers: { Authorization: `Bearer ${session.token}`, 'Content-Type': 'application/json' },
    data: { environment_id: ids![2], name: 'pg', engine: 'postgresql' },
  })
  expect(database.ok(), await database.text()).toBeTruthy()
  const db = (await database.json()) as { id?: string; database?: { id: string } }
  const dbID = db.id || db.database?.id
  await page.goto(`/databases/${dbID}`)
  await page.getByRole('button', { name: 'Backups' }).click()
  await expect(page.getByText(/backup/i).first()).toBeVisible()

  const auth = { Authorization: `Bearer ${session.token}`, 'Content-Type': 'application/json' }
  const server = await page.request.post('/api/v1/servers', {
    headers: auth,
    data: { name: `gone-${testInfo.project.name}`, ip: '203.0.113.10', port: 22, proxy_type: 'none' },
  })
  expect(server.ok(), await server.text()).toBeTruthy()
  const destinations = await page.request.get('/api/v1/destinations', { headers: auth })
  expect(destinations.ok(), await destinations.text()).toBeTruthy()
  const destBody = (await destinations.json()) as { destinations?: { id: string }[] }
  const destinationID = destBody.destinations?.[0]?.id
  expect(destinationID).toBeTruthy()
  const moved = await page.request.patch(`/api/v1/applications/${appID}`, {
    headers: auth,
    data: { destination_id: destinationID },
  })
  expect(moved.ok(), await moved.text()).toBeTruthy()
  const removed = await page.request.delete(`/api/v1/applications/${appID}`, {
    headers: auth,
    data: { confirmation_name: 'web', password: session.password },
  })
  expect(removed.ok(), await removed.text()).toBeTruthy()
  await page.goto('/dashboard')
  await expect(page.getByRole('heading', { name: 'Cleanup still running or failed' })).toBeVisible({ timeout: 20_000 })
  const retry = page.getByRole('button', { name: 'Retry cleanup' })
  if (await retry.count()) await retry.first().click()
  await page.getByRole('button', { name: 'Forget anyway' }).first().click()
  await page.getByRole('dialog').getByRole('button', { name: 'Forget anyway' }).click()

  await page.goto('/servers/new')
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  const serverName = page.getByLabel('Name').first()
  expect(await serverName.evaluate((el: HTMLInputElement) => !el.validity.valid)).toBe(true)

  await page.goto('/team')
  const inviteEmail = page.getByLabel('Invitation email')
  await inviteEmail.fill('not-an-email')
  await page.getByRole('button', { name: 'Invite' }).click()
  expect(await inviteEmail.evaluate((el: HTMLInputElement) => !el.validity.valid)).toBe(true)

  await page.goto('/settings')
  await expect(page.getByRole('heading').first()).toBeVisible()
  await axe(page)

  await page.goto('/notifications')
  await page.getByRole('button', { name: 'Webhook' }).click()
  await page.getByLabel('Webhook URL (POST)').fill('not-a-url')
  await expect(page.getByRole('alert')).toContainText('http')
})
