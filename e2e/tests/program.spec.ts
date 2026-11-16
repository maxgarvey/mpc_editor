import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, scanWorkspace, waitForHtmxOrTimeout } from './helpers';
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

test.describe('Program lifecycle', () => {
  test('clicking PGM in browser opens pad editor', async ({ page }) => {
    await page.goto('/');

    // Click the .pgm file in the browser panel
    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: '.pgm' }).first();
    const htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;

    // Pad grid should appear in detail panel
    const pads = page.locator('#detail-panel .pad-btn');
    await expect(pads).toHaveCount(16);

    // Pad params should be visible
    const params = page.locator('#pad-params');
    await expect(params).toBeVisible();
  });

  test('new program clears all pads', async ({ page }) => {
    await page.goto('/');

    // Open test.pgm which has sample references matching testdata WAVs
    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' });
    let htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;

    // Verify pads loaded (test.pgm should have at least one sample)
    const sampledPads = page.locator('#detail-panel .pad-btn.has-sample');
    const count = await sampledPads.count();
    expect(count).toBeGreaterThan(0);

    // Create a new program from the New modal (it opens in place of test.pgm)
    await page.getByRole('button', { name: 'New', exact: true }).click();
    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();
    await page.locator('#new-project-name').fill('blank01');
    await modal.getByRole('button', { name: 'Create Program' }).click();

    // Once the new program is shown, no pad has a sample
    await expect(page.locator('.detail-pgm[data-file-path$="blank01.pgm"]')).toBeVisible();
    await expect(sampledPads).toHaveCount(0);
  });

  test('profile switch updates page', async ({ page }) => {
    await page.goto('/');
    const profileSelect = page.locator('select[name="profile"]');
    await expect(profileSelect).toBeVisible();

    // Switch to MPC500
    await profileSelect.selectOption('MPC500');
    await page.waitForLoadState('networkidle');

    // Verify the profile is now MPC500
    await expect(profileSelect).toHaveValue('MPC500');
  });
});
