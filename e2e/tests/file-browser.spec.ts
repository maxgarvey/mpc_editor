import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, scanWorkspace, waitForHtmxOrTimeout } from './helpers';
import fs from 'fs';
import path from 'path';

let workspace: string;

test.beforeEach(async ({ page }) => {
  workspace = await setupWorkspace();
  await setWorkspace(page, workspace);
  await scanWorkspace(page);
});

test.afterEach(async () => {
  await cleanupWorkspace(workspace);
});

test.describe('File browser', () => {
  test('browser panel lists workspace files', async ({ page }) => {
    await page.goto('/');

    // Browser panel should be visible with entries
    const entries = page.locator('#file-nav .browser-entry');
    const count = await entries.count();
    expect(count).toBeGreaterThan(0);
  });

  test('browser shows PGM files', async ({ page }) => {
    await page.goto('/');

    const pgmEntries = page.locator('#file-nav .browser-entry', { hasText: '.pgm' });
    const count = await pgmEntries.count();
    expect(count).toBeGreaterThan(0);
  });

  test('browser shows WAV files', async ({ page }) => {
    await page.goto('/');

    const wavEntries = page.locator('#file-nav .browser-entry', { hasText: '.wav' });
    const count = await wavEntries.count();
    expect(count).toBeGreaterThan(0);
  });

  test('clicking WAV file shows audio info in detail panel', async ({ page }) => {
    await page.goto('/');

    const wavEntry = page.locator('#file-nav .browser-entry', { hasText: '.wav' }).first();
    if (await wavEntry.count() === 0) {
      test.skip();
      return;
    }

    const htmxDone = waitForHtmxOrTimeout(page);
    await wavEntry.click();
    await htmxDone;

    // Detail panel should show WAV info
    const detail = page.locator('#detail-panel');
    await expect(detail).toContainText('[WAV]');
  });

  test('mkdir creates a new directory', async ({ page }) => {
    // Use the API directly since the browse form uses HTMX
    await page.request.post('/workspace/mkdir', {
      form: { parent: '', name: 'new-folder', context: 'browse' },
    });

    // Verify the folder exists on disk
    const folderPath = path.join(workspace, 'new-folder');
    expect(fs.existsSync(folderPath)).toBe(true);
  });

  test('breadcrumbs are visible', async ({ page }) => {
    await page.goto('/');

    const breadcrumbs = page.locator('.nav-breadcrumbs');
    await expect(breadcrumbs).toBeVisible();

    // Should have at least the workspace root breadcrumb
    const links = breadcrumbs.locator('.breadcrumb-link');
    const count = await links.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });
});
