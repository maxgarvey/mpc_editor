import { Page } from '@playwright/test';
import fs from 'fs';
import os from 'os';
import path from 'path';

const PROJECT_ROOT = path.resolve(__dirname, '..', '..');
const TESTDATA_DIR = path.join(PROJECT_ROOT, 'testdata');

/** Create a temp directory with copies of testdata fixtures. */
export async function setupWorkspace(): Promise<string> {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mpc-e2e-'));
  const files = fs.readdirSync(TESTDATA_DIR);
  for (const file of files) {
    fs.copyFileSync(path.join(TESTDATA_DIR, file), path.join(tmpDir, file));
  }
  return tmpDir;
}

/** Remove the temp workspace directory. */
export async function cleanupWorkspace(dir: string): Promise<void> {
  fs.rmSync(dir, { recursive: true, force: true });
}

/** Set the workspace path via the server API. */
export async function setWorkspace(page: Page, workspacePath: string): Promise<void> {
  await page.request.post('/workspace/set', {
    form: { path: workspacePath },
  });
}

/** Trigger a workspace scan via the server API. */
export async function scanWorkspace(page: Page): Promise<void> {
  await page.request.post('/workspace/scan');
}

/** Open a .pgm program by posting to the server API. */
export async function openProgram(page: Page, pgmPath: string): Promise<void> {
  await page.request.post('/program/open', {
    form: { path: pgmPath },
  });
}

/**
 * Wait for an HTMX swap to complete.
 * Call this BEFORE triggering the action, then await the returned promise
 * AFTER triggering it. Or call after triggering if the swap is already in flight.
 */
export async function waitForHtmx(page: Page): Promise<void> {
  await page.evaluate(() => {
    return new Promise<void>((resolve) => {
      document.body.addEventListener('htmx:afterSettle', () => resolve(), { once: true });
    });
  });
}

/**
 * Wait for an HTMX swap with a timeout fallback.
 * Useful when you're not sure if an HTMX request will fire.
 */
export async function waitForHtmxOrTimeout(page: Page, timeoutMs = 3000): Promise<void> {
  await Promise.race([
    waitForHtmx(page),
    page.waitForTimeout(timeoutMs),
  ]);
}
