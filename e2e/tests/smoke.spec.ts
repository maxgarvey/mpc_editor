import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace } from './helpers';

let workspace: string;

test.beforeEach(async ({ page }) => {
  workspace = await setupWorkspace();
  await setWorkspace(page, workspace);
});

test.afterEach(async () => {
  await cleanupWorkspace(workspace);
});

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

  test('browser panel is visible', async ({ page }) => {
    await page.goto('/');
    const browser = page.locator('.browser-panel');
    await expect(browser).toBeVisible();
    await expect(browser).toContainText('Workspace');
  });

  test('detail panel shows welcome state', async ({ page }) => {
    await page.goto('/');
    const detail = page.locator('#detail-panel');
    await expect(detail).toBeVisible();
    await expect(detail).toContainText('Welcome to MPC Editor');
  });

  test('workspace files are listed in browser', async ({ page }) => {
    await page.goto('/');
    const entries = page.locator('#file-nav .browser-entry');
    const count = await entries.count();
    expect(count).toBeGreaterThan(0);
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
});
