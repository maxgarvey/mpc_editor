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

test.describe('WAV detail view', () => {
  test('clicking WAV shows waveform canvas and metadata', async ({ page }) => {
    await page.goto('/');

    const wavEntry = page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await wavEntry.click();
    await htmxDone;

    // Waveform canvas should be present
    const canvas = page.locator('#wav-waveform-canvas');
    await expect(canvas).toBeVisible();

    // Metadata should be visible
    await expect(page.locator('.detail-wav')).toContainText('Sample Rate');
    await expect(page.locator('.detail-wav')).toContainText('Channels');
    await expect(page.locator('.detail-wav')).toContainText('Duration');
  });

  test('play button uses correct AudioPlayer method', async ({ page }) => {
    await page.goto('/');

    const wavEntry = page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await wavEntry.click();
    await htmxDone;

    // Verify the play button exists and has correct onclick
    const playBtn = page.locator('.detail-wav button', { hasText: 'Play' });
    await expect(playBtn).toBeVisible();
    const onclick = await playBtn.getAttribute('onclick');
    expect(onclick).toContain('AudioPlayer.play');
    expect(onclick).toContain('/audio/file?path=');
    // Should NOT contain playURL
    expect(onclick).not.toContain('playURL');
  });

  test('WAV type badge is visible', async ({ page }) => {
    await page.goto('/');

    const wavEntry = page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await wavEntry.click();
    await htmxDone;

    await expect(page.locator('.detail-type')).toContainText('[WAV]');
  });
});
