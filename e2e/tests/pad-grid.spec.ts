import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, scanWorkspace, waitForHtmxOrTimeout } from './helpers';

let workspace: string;

test.beforeEach(async ({ page }) => {
  workspace = await setupWorkspace();
  await setWorkspace(page, workspace);
  await scanWorkspace(page);
  await page.goto('/');

  // Open the PGM file by clicking it in the browser panel
  const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: '.pgm' }).first();
  const htmxDone = waitForHtmxOrTimeout(page);
  await pgmEntry.click();
  await htmxDone;
});

test.afterEach(async () => {
  await cleanupWorkspace(workspace);
});

test.describe('Pad grid', () => {
  test('clicking a pad selects it and updates params', async ({ page }) => {
    const firstPad = page.locator('.pad-btn').first();
    const htmxDone = waitForHtmxOrTimeout(page);
    await firstPad.click();
    await htmxDone;

    // The pad should become selected
    await expect(firstPad).toHaveClass(/selected/);

    // Params panel should update
    const params = page.locator('#pad-params');
    await expect(params).toContainText('Pad');
  });

  test('bank switching loads different pads', async ({ page }) => {
    // Get initial pad grid content
    const initialHtml = await page.locator('#pad-grid').innerHTML();

    // Click bank B tab (use getByRole for exact match)
    const bankB = page.getByRole('button', { name: 'Bank B' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await bankB.click();
    await htmxDone;

    // Pad grid content should have changed (different pad indices)
    const newHtml = await page.locator('#pad-grid').innerHTML();
    expect(newHtml).not.toBe(initialHtml);
  });

  test('param tabs switch visible sections', async ({ page }) => {
    // Select a pad first
    const htmxDone = waitForHtmxOrTimeout(page);
    await page.locator('.pad-btn').first().click();
    await htmxDone;

    // Get all param tabs
    const tabs = page.locator('.param-tab');
    const tabCount = await tabs.count();
    expect(tabCount).toBeGreaterThanOrEqual(3);

    // Click each tab and verify its section becomes visible
    for (let i = 0; i < tabCount; i++) {
      await tabs.nth(i).click();
      await expect(tabs.nth(i)).toHaveClass(/active/);
    }
  });

  test('pad with sample shows sample name', async ({ page }) => {
    // beforeEach opens the first .pgm (chromatic.pgm, which has no samples),
    // so open test.pgm, whose pads name samples.
    const pgmEntry = page.locator('#file-nav .browser-entry', { hasText: 'test.pgm' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await pgmEntry.click();
    await htmxDone;
    await expect(page.locator('.detail-pgm[data-file-path$="test.pgm"]')).toBeVisible();

    const sampledPad = page.locator('#detail-panel .pad-btn.has-sample').first();
    await expect(sampledPad).toBeVisible();

    // The pad shows its sample name, and the title carries the same name
    const padName = sampledPad.locator('.pad-name');
    await expect(padName).toHaveText(/\S/);
    const name = (await padName.textContent())!.trim();
    await expect(sampledPad).toHaveAttribute('title', new RegExp('^' + name));
  });
});
