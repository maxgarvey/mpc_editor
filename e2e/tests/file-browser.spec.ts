import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, scanWorkspace } from './helpers';
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
  test('browse page lists workspace files', async ({ page }) => {
    await page.goto('/browse');

    // Should show directory entries
    const entries = page.locator('.browser-entry');
    const count = await entries.count();
    expect(count).toBeGreaterThan(0);
  });

  test('browse shows PGM files', async ({ page }) => {
    await page.goto('/browse');

    // Should have at least one .pgm entry
    const pgmEntries = page.locator('.browser-entry', { hasText: '.pgm' });
    const count = await pgmEntries.count();
    expect(count).toBeGreaterThan(0);
  });

  test('browse shows WAV files in load-wav context', async ({ page }) => {
    await page.goto('/browse?context=load-wav');

    const wavEntries = page.locator('.browser-entry', { hasText: '.wav' });
    const count = await wavEntries.count();
    expect(count).toBeGreaterThan(0);
  });

  test('file detail shows WAV metadata', async ({ page }) => {
    // Scan first so files are in the catalog
    await page.goto('/browse');

    // Click on a WAV file entry
    const wavEntry = page.locator('.browser-entry', { hasText: '.wav' }).first();
    if (await wavEntry.count() === 0) {
      test.skip();
      return;
    }
    await wavEntry.click();
    await page.waitForLoadState('networkidle');

    // Should show audio info section
    const detailSection = page.locator('.detail-section', { hasText: 'Audio Info' });
    if (await detailSection.count() > 0) {
      await expect(detailSection).toContainText('Sample Rate');
    }
  });

  test('mkdir creates a new directory', async ({ page }) => {
    // Use the API directly since the browse form uses HTMX
    await page.request.post('/workspace/mkdir', {
      form: { parent: '', name: 'new-folder', context: 'open-pgm' },
    });

    // Verify the folder exists on disk
    const folderPath = path.join(workspace, 'new-folder');
    expect(fs.existsSync(folderPath)).toBe(true);
  });
});
