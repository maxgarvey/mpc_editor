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

test.describe('Sample report', () => {
  test('Sample Report button generates txt file', async ({ page }) => {
    await page.goto('/');

    // Open the PGM file
    await page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' }).click();

    // Wait for PGM detail to load
    await expect(page.locator('.pad-btn').first()).toBeVisible();

    // Click Sample Report button
    await page.locator('button', { hasText: 'Sample Report' }).click();

    // Wait for status message
    await expect(page.locator('#report-status')).toContainText('Report saved to', { timeout: 10000 });

    // Verify the .txt file was created
    const txtPath = path.join(workspace, 'test_samples.txt');
    expect(fs.existsSync(txtPath)).toBe(true);

    // Verify content
    const content = fs.readFileSync(txtPath, 'utf-8');
    expect(content).toContain('Sample Report for test.pgm');
  });
});
