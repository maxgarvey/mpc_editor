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

test.describe('Layout', () => {
  test('landing page has browser and detail panels', async ({ page }) => {
    await page.goto('/');

    await expect(page.locator('.browser-panel')).toBeVisible();
    await expect(page.locator('#detail-panel')).toBeVisible();
    await expect(page.locator('.detail-welcome')).toBeVisible();
  });

  test('clicking PGM shows pad grid in detail panel', async ({ page }) => {
    await page.goto('/');

    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: '.pgm' }).first();
    const htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;

    // Pad grid and bank tabs should appear
    await expect(page.locator('#detail-panel .pad-btn').first()).toBeVisible();
    await expect(page.locator('#detail-panel .bank-tab').first()).toBeVisible();
  });

  test('clicking WAV shows audio metadata in detail panel', async ({ page }) => {
    await page.goto('/');

    const wavEntry = page.locator('#file-nav .browser-entry', { hasText: '.wav' }).first();
    if (await wavEntry.count() === 0) {
      test.skip();
      return;
    }

    const htmxDone = waitForHtmxOrTimeout(page);
    await wavEntry.click();
    await htmxDone;

    await expect(page.locator('#detail-panel')).toContainText('[WAV]');
  });

  test('switching between files swaps detail panel', async ({ page }) => {
    await page.goto('/');

    // Click a PGM first
    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: '.pgm' }).first();
    let htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;

    await expect(page.locator('#detail-panel .pad-btn').first()).toBeVisible();

    // Now click a WAV
    const wavEntry = page.locator('#file-nav .browser-entry', { hasText: '.wav' }).first();
    if (await wavEntry.count() === 0) {
      test.skip();
      return;
    }

    htmxDone = waitForHtmxOrTimeout(page);
    await wavEntry.click();
    await htmxDone;

    // Detail should now show WAV info, not pad grid
    await expect(page.locator('#detail-panel')).toContainText('[WAV]');
    await expect(page.locator('#detail-panel .pad-btn')).toHaveCount(0);
  });

  test('selected file gets active class in browser', async ({ page }) => {
    await page.goto('/');

    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: '.pgm' }).first();
    const htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;

    // The clicked entry should have the active class
    await expect(pgmEntry).toHaveClass(/active/);
  });

  test('browser persists when detail changes', async ({ page }) => {
    await page.goto('/');

    // Count browser entries before clicking
    const entriesBefore = await page.locator('#file-nav .browser-entry').count();
    expect(entriesBefore).toBeGreaterThan(0);

    // Click a file to change the detail panel
    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: '.pgm' }).first();
    const htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;

    // Browser entries should still be there
    const entriesAfter = await page.locator('#file-nav .browser-entry').count();
    expect(entriesAfter).toBe(entriesBefore);
  });

  test('slicer link navigates to standalone page', async ({ page }) => {
    await page.goto('/');

    await page.locator('a', { hasText: 'Slicer' }).click();
    await page.waitForLoadState('networkidle');

    await expect(page).toHaveTitle(/Slicer/);
  });

  test('batch link navigates to standalone page', async ({ page }) => {
    await page.goto('/');

    await page.locator('a', { hasText: 'Batch' }).click();
    await page.waitForLoadState('networkidle');

    await expect(page).toHaveTitle(/Batch/);
  });

  test('new folder form is visible in browser', async ({ page }) => {
    await page.goto('/');

    const mkdirForm = page.locator('.nav-mkdir');
    await expect(mkdirForm).toBeVisible();
    await expect(mkdirForm.locator('input[name="name"]')).toBeVisible();
  });
});
