import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, openProgram } from './helpers';
import path from 'path';

let workspace: string;

test.beforeEach(async ({ page }) => {
  workspace = await setupWorkspace();
  await setWorkspace(page, workspace);
});

test.afterEach(async () => {
  await cleanupWorkspace(workspace);
});

test.describe('Program lifecycle', () => {
  test('new program clears all pads', async ({ page }) => {
    // Open a program first so pads have samples
    await openProgram(page, path.join(workspace, 'test.pgm'));
    await page.goto('/');

    // Click New and accept the confirmation dialog
    page.on('dialog', (dialog) => dialog.accept());
    await page.locator('button', { hasText: 'New' }).click();

    // Wait for page to reload/update
    await page.waitForLoadState('networkidle');

    // No pads should have samples
    const sampledPads = page.locator('.pad-btn.has-sample');
    await expect(sampledPads).toHaveCount(0);
  });

  test('open program shows samples on pads', async ({ page }) => {
    await openProgram(page, path.join(workspace, 'test.pgm'));
    await page.goto('/');

    // At least one pad should have a sample assigned
    const sampledPads = page.locator('.pad-btn.has-sample');
    const count = await sampledPads.count();
    expect(count).toBeGreaterThan(0);
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
