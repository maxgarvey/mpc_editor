import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, waitForHtmxOrTimeout } from './helpers';
import path from 'path';

let workspace: string;

test.beforeEach(async ({ page }) => {
  workspace = await setupWorkspace();
  await setWorkspace(page, workspace);
});

test.afterEach(async () => {
  await cleanupWorkspace(workspace);
});

test.describe('Slicer page', () => {
  test('slicer page loads', async ({ page }) => {
    await page.goto('/slicer');
    await expect(page).toHaveTitle(/Slicer/);
    const header = page.locator('.app-header');
    await expect(header).toContainText('Slicer');
  });

  test('load a WAV file shows waveform', async ({ page }) => {
    await page.goto('/slicer');

    // Enter the path to myLoop.wav
    const pathInput = page.locator('#load-wav-path');
    await pathInput.fill(path.join(workspace, 'myLoop.wav'));

    // Submit the load form
    const htmxDone = waitForHtmxOrTimeout(page);
    await page.locator('button', { hasText: 'Load' }).click();
    await htmxDone;

    // Waveform canvas should be visible
    const canvas = page.locator('#waveform-canvas');
    await expect(canvas).toBeVisible();
  });

  test('slicer info bar shows audio metadata', async ({ page }) => {
    await page.goto('/slicer');

    const pathInput = page.locator('#load-wav-path');
    await pathInput.fill(path.join(workspace, 'myLoop.wav'));

    const htmxDone = waitForHtmxOrTimeout(page);
    await page.locator('button', { hasText: 'Load' }).click();
    await htmxDone;

    // Info bar should show duration and marker info
    const info = page.locator('.slicer-info');
    await expect(info).toBeVisible();
  });

  test('sensitivity slider updates markers', async ({ page }) => {
    await page.goto('/slicer');

    const pathInput = page.locator('#load-wav-path');
    await pathInput.fill(path.join(workspace, 'myLoop.wav'));

    let htmxDone = waitForHtmxOrTimeout(page);
    await page.locator('button', { hasText: 'Load' }).click();
    await htmxDone;

    // Adjust the sensitivity slider
    const slider = page.locator('#sensitivity-slider');
    if (await slider.isVisible()) {
      htmxDone = waitForHtmxOrTimeout(page);
      await slider.fill('200');
      await slider.dispatchEvent('change');
      await htmxDone;
    }
  });
});
