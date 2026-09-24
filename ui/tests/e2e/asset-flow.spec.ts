import { expect, test, type Page } from '@playwright/test'
import { base, signIn } from './helpers'

// Quickstart §4 flow for the asset remote on the kit at the three reference
// widths. Needs a full platform; skips without operator credentials.
const password = process.env.E2E_OPERATOR_PASSWORD ?? ''
const email = process.env.E2E_OPERATOR_EMAIL ?? 'ops@example.org'
const viewports = [
  { name: 'phone', width: 320, height: 640 },
  { name: 'tablet', width: 768, height: 1024 },
  { name: 'desktop', width: 1280, height: 800 },
]

function watchCsp(page: Page): string[] {
  const violations: string[] = []
  void page.addInitScript(() => {
    document.addEventListener('securitypolicyviolation', (e) => console.error('CSP:' + (e as SecurityPolicyViolationEvent).violatedDirective))
  })
  page.on('console', (m) => { if (m.text().startsWith('CSP:')) violations.push(m.text()) })
  return violations
}

async function openNav(page: Page, group: string, entry: string): Promise<void> {
  const burger = page.getByRole('button', { name: 'Open navigation' })
  if (await burger.isVisible()) await burger.click()
  const g = page.getByTestId('nav-group-' + group)
  if ((await g.getAttribute('aria-expanded')) !== 'true') await g.click()
  await page.getByTestId('nav-' + group).filter({ hasText: entry }).first().click()
}

test.describe('asset remote', () => {
  test.skip(!password, 'E2E_OPERATOR_PASSWORD not set')

  for (const vp of viewports) {
    test(`${vp.name}: create → assign → unassign → history, document, sync preview, dashboard`, async ({ page }) => {
      await page.setViewportSize({ width: vp.width, height: vp.height })
      const violations = watchCsp(page)
      await page.goto(base + '/')
      await signIn(page, email, password)
      await openNav(page, 'asset', 'Assets')
      await expect(page.locator('main h1')).toHaveText('Assets')
      // Create
      const tag = 'E2E-' + vp.name.toUpperCase() + '-' + Date.now().toString(36)
      await page.getByRole('button', { name: 'New asset' }).click()
      const dialog = page.getByRole('dialog')
      await dialog.locator('[data-field=name]').fill('E2E laptop ' + vp.name)
      await dialog.locator('[data-field=asset_tag]').fill(tag)
      await dialog.locator('[data-field=purchase_cost]').fill('1200')
      await dialog.getByRole('button', { name: 'Save' }).click()
      await expect(dialog).toBeHidden()
      await expect(page.locator('main')).toContainText(tag)
      // Detail: assign, unassign, history
      await page.locator('main').getByText(tag, { exact: true }).first().click()
      await expect(page.locator('main h1')).toHaveText(tag)
      await page.getByRole('button', { name: 'Assign', exact: true }).click()
      const assign = page.getByRole('dialog')
      await assign.locator('[data-field=user_id]').fill('')
      await assign.locator('[data-field=user_id]').press('ArrowDown')
      await assign.locator('[role=option]').first().click()
      await assign.getByRole('button', { name: 'Assign', exact: true }).click()
      await expect(assign).toBeHidden()
      await expect(page.locator('main')).toContainText('assigned')
      await page.getByRole('button', { name: 'Unassign', exact: true }).click()
      await page.getByRole('dialog').getByRole('button', { name: 'Unassign', exact: true }).click()
      await expect(page.getByRole('dialog')).toBeHidden()
      await expect(page.locator('main')).toContainText('unassigned')
      // Document upload
      await page.locator('input[type=file][id$=file]').last().setInputFiles({ name: 'note.txt', mimeType: 'text/plain', buffer: Buffer.from('e2e') })
      await page.getByRole('button', { name: 'Upload' }).last().click()
      await expect(page.locator('main')).toContainText('note.txt')
      // Sync preview + dashboard
      await openNav(page, 'asset', 'Inventory sync')
      await page.getByRole('button', { name: 'Preview' }).click()
      await expect(page.locator('main')).toContainText(/Hosts|No inventory hosts/)
      await openNav(page, 'asset', 'Dashboard')
      await expect(page.locator('.stat-tile').first()).toBeVisible()
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
      expect(overflow, 'no horizontal page scroll').toBeLessThanOrEqual(0)
      expect(await page.locator('main [style]').count(), 'no inline styles').toBe(0)
      expect(violations).toEqual([])
    })
  }
})
