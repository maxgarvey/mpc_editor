import { test, expect, Page } from '@playwright/test';
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

/**
 * Click a file in the browser and wait for #tags-section to appear.
 * Re-sets workspace and re-scans first to handle parallel worker contention
 * (another worker may have changed the active workspace on the shared server).
 */
async function clickFileAndWaitForTags(page: Page, ws: string, fileName: string) {
  // Re-set workspace to counter parallel worker overwriting it
  await page.request.post('/workspace/set', { form: { path: ws } });
  await page.request.post('/workspace/scan');

  // Reload browser panel with correct workspace
  await page.goto('/');

  const entry = page.locator('#file-nav .browser-entry', { hasText: fileName });
  const tagsSection = page.locator('#tags-section');
  for (let attempt = 0; attempt < 6; attempt++) {
    await entry.click();
    try {
      await expect(tagsSection).toBeVisible({ timeout: 2000 });
      return;
    } catch {
      await page.waitForTimeout(1000);
    }
  }
  await entry.click();
  await expect(tagsSection).toBeVisible({ timeout: 5000 });
}

test.describe('File tagging', () => {
  // Auto-tags can be flaky under parallel workers due to SQLITE_BUSY during scans
  test.describe.configure({ retries: 1 });
  test('WAV detail shows auto-tags after scan', async ({ page }) => {
    await page.goto('/');

    await clickFileAndWaitForTags(page, workspace, 'chh.wav');

    const tagsSection = page.locator('#tags-section');
    await expect(tagsSection.locator('.tag-chip.tag-auto').first()).toBeVisible({ timeout: 10000 });
  });

  test('adding a free-form tag', async ({ page }) => {
    await clickFileAndWaitForTags(page, workspace, 'chh.wav');

    const tagsSection = page.locator('#tags-section');
    await tagsSection.locator('.tag-input').fill('kick');
    await tagsSection.locator('button[type="submit"]').click();

    await expect(page.locator('#tags-section .tag-chip', { hasText: 'kick' })).toBeVisible({ timeout: 5000 });
  });

  test('adding a key:value tag', async ({ page }) => {
    await clickFileAndWaitForTags(page, workspace, 'chh.wav');

    const tagsSection = page.locator('#tags-section');
    await tagsSection.locator('.tag-input').fill('genre:house');
    await tagsSection.locator('button[type="submit"]').click();

    await expect(page.locator('#tags-section .tag-chip', { hasText: 'house' })).toBeVisible({ timeout: 10000 });

    const tagChip = page.locator('#tags-section .tag-chip', { hasText: 'house' });
    await expect(tagChip.locator('.tag-key')).toContainText('genre:');
    await expect(tagChip.locator('.tag-value')).toContainText('house');
  });

  test('removing a tag', async ({ page }) => {
    await clickFileAndWaitForTags(page, workspace, 'chh.wav');

    const tagsSection = page.locator('#tags-section');
    await tagsSection.locator('.tag-input').fill('removeme');
    await tagsSection.locator('button[type="submit"]').click();
    await expect(page.locator('#tags-section .tag-chip', { hasText: 'removeme' })).toBeVisible({ timeout: 5000 });

    const chip = page.locator('#tags-section .tag-chip', { hasText: 'removeme' });
    await chip.locator('.tag-remove').click();

    await expect(page.locator('#tags-section .tag-chip', { hasText: 'removeme' })).toHaveCount(0);
  });

  test('SEQ detail shows auto-tags for BPM', async ({ page }) => {
    await clickFileAndWaitForTags(page, workspace, 'test.seq');

    const tagsSection = page.locator('#tags-section');
    await expect(tagsSection.locator('.tag-chip', { hasText: 'bpm' })).toBeVisible({ timeout: 10000 });
  });

  test('tags persist across tab switches', async ({ page }) => {
    await clickFileAndWaitForTags(page, workspace, 'chh.wav');

    const tagsSection = page.locator('#tags-section');
    await tagsSection.locator('.tag-input').fill('persist-test');
    await tagsSection.locator('button[type="submit"]').click();
    await expect(page.locator('#tags-section .tag-chip', { hasText: 'persist-test' })).toBeVisible({ timeout: 5000 });

    // Open another file
    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();
    await expect(page.locator('.pad-btn').first()).toBeVisible();

    // Switch back to WAV tab
    await page.locator('.detail-tab', { hasText: 'chh.wav' }).click();

    await expect(page.locator('#tags-section .tag-chip', { hasText: 'persist-test' })).toBeVisible({ timeout: 5000 });
  });
});
