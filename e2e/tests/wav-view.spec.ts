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

  test('transport buttons drive the WAV detail player', async ({ page }) => {
    await page.goto('/');

    const wavEntry = page.locator('#file-nav .browser-entry', { hasText: 'chh.wav' });
    const htmxDone = waitForHtmxOrTimeout(page);
    await wavEntry.click();
    await htmxDone;

    // Play/Pause and Stop call WavDetailPlayer, which reads the file from the
    // panel's data-rel-path.
    const playBtn = page.locator('#wav-play-pause-btn');
    await expect(playBtn).toBeVisible();
    expect(await playBtn.getAttribute('onclick')).toBe('WavDetailPlayer.toggle()');
    const stopBtn = page.locator('.detail-wav button', { hasText: 'Stop' });
    expect(await stopBtn.getAttribute('onclick')).toBe('WavDetailPlayer.stop()');
    await expect(page.locator('.detail-wav')).toHaveAttribute('data-rel-path', /chh\.wav$/);
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
