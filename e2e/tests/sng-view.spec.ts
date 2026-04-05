import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, scanWorkspace, waitForHtmxOrTimeout } from './helpers';

let workspace: string;

test.beforeEach(async ({ page }) => {
  workspace = await setupWorkspace();
  await setWorkspace(page, workspace);
  await scanWorkspace(page);
});

test.afterEach(async () => {
  await cleanupWorkspace(workspace);
});

test.describe('SNG detail view', () => {
  test('clicking SNG shows song metadata placeholder', async ({ page }) => {
    await page.goto('/');

    const sngEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.sng' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await sngEntry.click();
    await htmxDone;

    // SNG type badge
    await expect(page.locator('.detail-type')).toContainText('[SNG]');

    // Should show the placeholder message
    await expect(page.locator('.detail-sng')).toContainText('Song editing is planned');
  });

  test('SNG view shows file size', async ({ page }) => {
    await page.goto('/');

    const sngEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.sng' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await sngEntry.click();
    await htmxDone;

    await expect(page.locator('.detail-sng')).toContainText('bytes');
  });
});
