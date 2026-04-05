import { test, expect } from '@playwright/test';

test.describe('Smoke tests', () => {
  test('page loads with correct title', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveTitle(/MPC Editor/);
  });

  test('header is visible', async ({ page }) => {
    await page.goto('/');
    const header = page.locator('.app-header');
    await expect(header).toBeVisible();
    await expect(header).toContainText('MPC');
    await expect(header).toContainText('Editor');
  });

  test('pad grid renders 16 pads', async ({ page }) => {
    await page.goto('/');
    const pads = page.locator('.pad-btn');
    await expect(pads).toHaveCount(16);
  });

  test('bank tabs are visible', async ({ page }) => {
    await page.goto('/');
    const tabs = page.locator('.bank-tab');
    await expect(tabs).toHaveCount(4);
    await expect(tabs.first()).toContainText('A');
  });

  test('static assets load without errors', async ({ page }) => {
    const errors: string[] = [];
    page.on('response', (response) => {
      if (response.url().includes('/static/') && response.status() >= 400) {
        errors.push(`${response.status()} ${response.url()}`);
      }
    });
    await page.goto('/');
    expect(errors).toHaveLength(0);
  });

  test('pad params panel is present', async ({ page }) => {
    await page.goto('/');
    const params = page.locator('#pad-params');
    await expect(params).toBeVisible();
  });
});
