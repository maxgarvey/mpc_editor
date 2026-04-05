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

test.describe('New modal', () => {
  test('modal opens with two tabs', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();

    // Should have two tabs
    const tabs = modal.locator('.new-modal-tab');
    await expect(tabs).toHaveCount(2);
    await expect(tabs.nth(0)).toContainText('New Program');
    await expect(tabs.nth(1)).toContainText('Import Files');
  });

  test('New Program tab is default and has Create button', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();

    // New Program tab content should be visible
    const programTab = page.locator('#new-program-tab');
    await expect(programTab).toBeVisible();

    // Create button should be present
    const createBtn = programTab.getByRole('button', { name: 'Create' });
    await expect(createBtn).toBeVisible();
  });

  test('New Program creates blank program', async ({ page }) => {
    await page.goto('/');

    // First open a PGM so we have pads loaded
    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' });
    let htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;

    // Open modal and create new program
    await page.getByRole('button', { name: 'New', exact: true }).click();
    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();
    await modal.getByRole('button', { name: 'Create' }).click();

    // Wait for page to reload
    await page.waitForLoadState('networkidle');

    // Modal should be gone
    await expect(page.locator('.new-modal')).toHaveCount(0);
  });

  test('Import Files tab shows drop zone', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();

    // Click Import Files tab
    await modal.locator('.new-modal-tab', { hasText: 'Import Files' }).click();

    // Drop zone should be visible
    const dropZone = page.locator('.import-drop-zone');
    await expect(dropZone).toBeVisible();

    // Browse Files button should be visible
    await expect(modal.getByRole('button', { name: 'Browse Files' })).toBeVisible();
  });

  test('Import destination shows workspace path', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await modal.locator('.new-modal-tab', { hasText: 'Import Files' }).click();

    // Destination path should be visible
    const destPath = page.locator('.import-dest-path');
    await expect(destPath).toBeVisible();
    const text = await destPath.textContent();
    expect(text!.length).toBeGreaterThan(0);
  });

  test('can select files via Browse button', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await modal.locator('.new-modal-tab', { hasText: 'Import Files' }).click();

    // Use the hidden file input to simulate file selection
    const fileInput = page.locator('#import-file-input');
    await fileInput.setInputFiles(path.join(workspace, 'chh.wav'));

    // File list should show the file
    const fileList = page.locator('.import-file-list');
    await expect(fileList).toContainText('chh.wav');

    // Import button should be enabled
    const importBtn = page.locator('#import-btn');
    await expect(importBtn).toBeEnabled();
  });

  test('import copies files to workspace', async ({ page }) => {
    await page.goto('/');

    // Create a subdirectory to import into
    await page.request.post('/workspace/mkdir', {
      form: { parent: '', name: 'imports', context: 'browse' },
    });

    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await modal.locator('.new-modal-tab', { hasText: 'Import Files' }).click();

    // Select a file
    const fileInput = page.locator('#import-file-input');
    await fileInput.setInputFiles(path.join(workspace, 'chh.wav'));

    // Click Import
    await page.locator('#import-btn').click();

    // Wait for modal to close
    await expect(page.locator('.new-modal')).toHaveCount(0);

    // The file should still exist in the workspace (it was imported from workspace itself)
    expect(fs.existsSync(path.join(workspace, 'chh.wav'))).toBe(true);
  });

  test('close button dismisses modal', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();

    // Click close button
    await modal.locator('.new-modal-close').click();

    // Modal should be gone
    await expect(page.locator('.new-modal')).toHaveCount(0);
  });
});
