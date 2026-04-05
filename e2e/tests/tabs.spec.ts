import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, scanWorkspace } from './helpers';

let workspace: string;

test.beforeEach(async ({ page }) => {
  workspace = await setupWorkspace();
  await setWorkspace(page, workspace);
  await scanWorkspace(page);
});

test.afterEach(async () => {
  await cleanupWorkspace(workspace);
});

test.describe('Tab system', () => {
  test('opening a file creates a tab', async ({ page }) => {
    await page.goto('/');

    // Click a PGM file
    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();

    // Wait for tab to appear
    const tab = page.locator('.detail-tab');
    await expect(tab).toHaveCount(1);
    await expect(tab.first()).toContainText('test.pgm');
    await expect(tab.first()).toHaveClass(/active/);
  });

  test('opening a second file creates a second tab', async ({ page }) => {
    await page.goto('/');

    // Open PGM
    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(1);

    // Open WAV
    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(2);

    // Second tab should be active
    const tabs = page.locator('.detail-tab');
    await expect(tabs.nth(1)).toHaveClass(/active/);
    await expect(tabs.nth(0)).not.toHaveClass(/active/);
  });

  test('clicking a tab switches content', async ({ page }) => {
    await page.goto('/');

    // Open PGM then WAV
    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(1);

    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(2);

    // WAV content should be showing
    await expect(page.locator('.detail-tab-content')).toContainText('[WAV]');

    // Click the PGM tab
    await page.locator('.detail-tab', { hasText: 'test.pgm' }).click();

    // Should now show PGM content
    await expect(page.locator('.detail-tab-content .pad-btn').first()).toBeVisible();
    await expect(page.locator('.detail-tab', { hasText: 'test.pgm' })).toHaveClass(/active/);
  });

  test('closing a tab removes it and activates another', async ({ page }) => {
    await page.goto('/');

    // Open two files
    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(1);

    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(2);

    // Close the WAV tab (active)
    await page.locator('.detail-tab', { hasText: 'chh.wav' }).locator('.detail-tab-close').click();

    // Should have one tab left
    await expect(page.locator('.detail-tab')).toHaveCount(1);
    await expect(page.locator('.detail-tab', { hasText: 'test.pgm' })).toHaveClass(/active/);
  });

  test('closing last tab shows welcome', async ({ page }) => {
    await page.goto('/');

    // Open a file
    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(1);

    // Close it
    await page.locator('.detail-tab-close').click();

    // No tabs, welcome message
    await expect(page.locator('.detail-tab')).toHaveCount(0);
    await expect(page.locator('.detail-welcome')).toBeVisible();
  });

  test('active tab highlights browser entry', async ({ page }) => {
    await page.goto('/');

    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(1);

    // Browser entry should have active class
    const entry = page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' });
    await expect(entry).toHaveClass(/active/);
  });

  test('open tabs show indicator in browser', async ({ page }) => {
    await page.goto('/');

    // Open two files
    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(1);

    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(2);

    // WAV is active, PGM should have 'open' class
    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' });
    await expect(pgmEntry).toHaveClass(/open/);

    // WAV should have 'active' class (not 'open')
    const wavEntry = page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' });
    await expect(wavEntry).toHaveClass(/active/);
  });

  test('re-clicking same file activates existing tab', async ({ page }) => {
    await page.goto('/');

    // Click the same file twice
    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();
    await expect(page.locator('.detail-tab')).toHaveCount(1);

    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();

    // Should still be just one tab
    await expect(page.locator('.detail-tab')).toHaveCount(1);
  });
});
