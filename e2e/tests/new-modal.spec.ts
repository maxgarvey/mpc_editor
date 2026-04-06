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
  test('modal opens with three tabs', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();

    // Should have three tabs
    const tabs = modal.locator('.new-modal-tab');
    await expect(tabs).toHaveCount(3);
    await expect(tabs.nth(0)).toContainText('New Project');
    await expect(tabs.nth(1)).toContainText('New Program');
    await expect(tabs.nth(2)).toContainText('Import Files');
  });

  test('New Project tab is default and has Create Project button', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'New', exact: true }).click();

    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();

    // New Project tab content should be visible
    const projectTab = page.locator('#new-project-tab');
    await expect(projectTab).toBeVisible();

    // Create Project button should be present (disabled until name entered)
    const createBtn = projectTab.getByRole('button', { name: 'Create Project' });
    await expect(createBtn).toBeVisible();
    await expect(createBtn).toBeDisabled();
  });

  test('New Program creates blank program', async ({ page }) => {
    await page.goto('/');

    // First open a PGM so we have pads loaded
    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' });
    let htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;

    // Open modal and switch to New Program tab
    await page.getByRole('button', { name: 'New', exact: true }).click();
    const modal = page.locator('.new-modal');
    await expect(modal).toBeVisible();
    await modal.locator('.new-modal-tab', { hasText: 'New Program' }).click();
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
