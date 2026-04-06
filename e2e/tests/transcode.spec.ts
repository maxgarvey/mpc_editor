import { test, expect } from '@playwright/test';
import { setupWorkspace, cleanupWorkspace, setWorkspace, scanWorkspace } from './helpers';
import fs from 'fs';
import path from 'path';

const PROJECT_ROOT = path.resolve(__dirname, '..', '..');
const TESTDATA_DIR = path.join(PROJECT_ROOT, 'testdata');

let workspace: string;

test.beforeEach(async ({ page }) => {
  workspace = await setupWorkspace();
  await setWorkspace(page, workspace);
  await scanWorkspace(page);
});

test.afterEach(async () => {
  await cleanupWorkspace(workspace);
});

test.describe('Audio transcoding', () => {
  test('importing an MP3 transcodes it to WAV', async ({ page }) => {
    await page.goto('/');

    // Use the workspace/import API directly with the MP3 file
    const mp3Path = path.join(TESTDATA_DIR, 'test_audio.mp3');
    expect(fs.existsSync(mp3Path)).toBe(true);

    // Post the MP3 file via the API
    const response = await page.request.post('/workspace/import', {
      multipart: {
        dest: workspace,
        files: {
          name: 'test_audio.mp3',
          mimeType: 'audio/mpeg',
          buffer: fs.readFileSync(mp3Path),
        },
      },
    });

    expect(response.ok()).toBe(true);
    const data = await response.json();
    expect(data.imported).toBe(1);
    expect(data.transcoded).toBe(1);

    // The transcoded WAV file should now exist in the workspace
    const wavPath = path.join(workspace, 'test_audio.wav');
    expect(fs.existsSync(wavPath)).toBe(true);

    // Verify the WAV is a valid RIFF file
    const header = Buffer.alloc(4);
    const fd = fs.openSync(wavPath, 'r');
    fs.readSync(fd, header, 0, 4, 0);
    fs.closeSync(fd);
    expect(header.toString('ascii')).toBe('RIFF');
  });

  test('import modal shows audio formats in accepted types', async ({ page }) => {
    await page.goto('/');

    // Open the new modal
    await page.locator('button', { hasText: /^New$/ }).click();

    // Switch to import tab
    await page.locator('.new-modal-tab', { hasText: 'Import' }).click();

    // The hint should mention mp3
    await expect(page.locator('.import-drop-zone-hint')).toContainText('.mp3');
  });
});
