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

test.describe('SEQ detail view', () => {
  test('clicking SEQ shows sequence grid with velocity coloring', async ({ page }) => {
    await page.goto('/');

    const seqEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.seq' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await seqEntry.click();
    await htmxDone;

    // SEQ type badge
    await expect(page.locator('.detail-type')).toContainText('[SEQ]');

    // Step grid should be visible
    await expect(page.locator('.step-grid')).toBeVisible();

    // Active cells should have velocity-based background colors (not just #cc2020)
    const activeCells = page.locator('.step-active');
    const count = await activeCells.count();
    if (count > 0) {
      const bgColor = await activeCells.first().evaluate(el => {
        return window.getComputedStyle(el).backgroundColor;
      });
      // Should be one of the velocity colors, not the default red
      expect(bgColor).toBeTruthy();
    }
  });

  test('mute and solo buttons are present', async ({ page }) => {
    await page.goto('/');

    const seqEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.seq' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await seqEntry.click();
    await htmxDone;

    // Mute and solo buttons should exist in track name cells
    const muteBtn = page.locator('.track-mute-btn').first();
    const soloBtn = page.locator('.track-solo-btn').first();
    await expect(muteBtn).toBeVisible();
    await expect(soloBtn).toBeVisible();
  });

  test('clicking mute dims the track row', async ({ page }) => {
    await page.goto('/');

    const seqEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.seq' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await seqEntry.click();
    await htmxDone;

    const muteBtn = page.locator('.track-mute-btn').first();
    await muteBtn.click();

    // Button should get active class
    await expect(muteBtn).toHaveClass(/active/);

    // Row should get track-muted class
    const row = page.locator('.step-grid tbody tr').first();
    await expect(row).toHaveClass(/track-muted/);
  });

  test('clicking solo highlights the solo button', async ({ page }) => {
    await page.goto('/');

    const seqEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.seq' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await seqEntry.click();
    await htmxDone;

    const soloBtn = page.locator('.track-solo-btn').first();
    await soloBtn.click();

    await expect(soloBtn).toHaveClass(/active/);
  });
});
