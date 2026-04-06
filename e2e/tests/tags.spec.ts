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

test.describe('File tagging', () => {
  test('WAV detail shows auto-tags after scan', async ({ page }) => {
    await page.goto('/');

    // Click the WAV file to open it
    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();

    // Wait for tags section to appear
    const tagsSection = page.locator('#tags-section');
    await expect(tagsSection).toBeVisible();

    // Auto-tags depend on scanner completing; may need a re-click if SQLITE_BUSY delayed it
    await expect(tagsSection.locator('.tag-chip.tag-auto').first()).toBeVisible({ timeout: 10000 });
  });

  test('adding a free-form tag', async ({ page }) => {
    await page.goto('/');

    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();

    const tagsSection = page.locator('#tags-section');
    await expect(tagsSection).toBeVisible();

    // Type a tag and submit
    await tagsSection.locator('.tag-input').fill('kick');
    await tagsSection.locator('button[type="submit"]').click();

    // Wait for HTMX swap to complete
    await expect(page.locator('#tags-section .tag-chip', { hasText: 'kick' })).toBeVisible();
  });

  test('adding a key:value tag', async ({ page }) => {
    await page.goto('/');

    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();

    const tagsSection = page.locator('#tags-section');
    await expect(tagsSection).toBeVisible();

    // Type a key:value tag
    await tagsSection.locator('.tag-input').fill('genre:house');
    await tagsSection.locator('button[type="submit"]').click();

    // Wait for HTMX swap — the tags-section gets replaced
    await expect(page.locator('#tags-section .tag-chip', { hasText: 'house' })).toBeVisible({ timeout: 10000 });

    // Should show the key styled differently
    const tagChip = page.locator('#tags-section .tag-chip', { hasText: 'house' });
    await expect(tagChip.locator('.tag-key')).toContainText('genre:');
    await expect(tagChip.locator('.tag-value')).toContainText('house');
  });

  test('removing a tag', async ({ page }) => {
    await page.goto('/');

    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();

    const tagsSection = page.locator('#tags-section');
    await expect(tagsSection).toBeVisible();

    // Add a tag first
    await tagsSection.locator('.tag-input').fill('removeme');
    await tagsSection.locator('button[type="submit"]').click();
    await expect(page.locator('#tags-section .tag-chip', { hasText: 'removeme' })).toBeVisible();

    // Remove it
    const chip = page.locator('#tags-section .tag-chip', { hasText: 'removeme' });
    await chip.locator('.tag-remove').click();

    // Should be gone
    await expect(page.locator('#tags-section .tag-chip', { hasText: 'removeme' })).toHaveCount(0);
  });

  test('SEQ detail shows auto-tags for BPM', async ({ page }) => {
    await page.goto('/');

    await page.locator('#file-nav .browser-entry', { hasText: 'test.seq' }).click();

    // Wait for SEQ detail to load
    await expect(page.locator('.detail-seq')).toBeVisible();

    // Tags section should be present with auto-tags
    const tagsSection = page.locator('#tags-section');
    await expect(tagsSection).toBeVisible({ timeout: 10000 });

    // Should have bpm auto-tag
    await expect(tagsSection.locator('.tag-chip', { hasText: 'bpm' })).toBeVisible({ timeout: 10000 });
  });

  test('tags persist across tab switches', async ({ page }) => {
    await page.goto('/');

    // Open WAV file
    await page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' }).click();
    const tagsSection = page.locator('#tags-section');
    await expect(tagsSection).toBeVisible();

    // Add a custom tag
    await tagsSection.locator('.tag-input').fill('persist-test');
    await tagsSection.locator('button[type="submit"]').click();
    await expect(page.locator('#tags-section .tag-chip', { hasText: 'persist-test' })).toBeVisible();

    // Open another file
    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();
    await expect(page.locator('.pad-btn').first()).toBeVisible();

    // Switch back to WAV tab
    await page.locator('.detail-tab', { hasText: 'chh.wav' }).click();

    // Tag should still be there (re-fetched from DB)
    await expect(page.locator('#tags-section .tag-chip', { hasText: 'persist-test' })).toBeVisible();
  });
});
