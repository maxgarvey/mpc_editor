import { test, expect } from '@playwright/test';

test.describe('Sequence viewer', () => {
  test('shows error for missing path', async ({ page }) => {
    await page.goto('/sequence');
    const error = page.locator('.sequence-error');
    await expect(error).toBeVisible();
    await expect(error).toContainText('path is required');
  });

  test('shows error for invalid file', async ({ page }) => {
    await page.goto('/sequence?path=/nonexistent/file.seq');
    const error = page.locator('.sequence-error');
    await expect(error).toBeVisible();
  });

  test('page structure is correct', async ({ page }) => {
    // Even with an error, the page layout should render
    await page.goto('/sequence');
    await expect(page).toHaveTitle(/Sequence/);
    const header = page.locator('.app-header');
    await expect(header).toContainText('Sequence');
  });
});
