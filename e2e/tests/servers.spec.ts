import { test, expect } from '../fixtures/auth';

const serverName = 'e2e-test-' + Date.now().toString(36);

test.describe.serial('Game Servers', () => {
  test('user can create a game server', async ({ userPage: page }) => {
    await page.goto('/servers/create');

    // Step 1: Game selection
    await expect(
      page.getByRole('heading', { name: 'Create Server' }),
    ).toBeVisible();

    // Select Minecraft from the game list
    await page.getByText('Minecraft Java Edition').click();

    // Step 2: Configure server (URL now has ?game=minecraft)
    await expect(page).toHaveURL(/\/servers\/create\?game=minecraft/);

    // Fill server name
    await page.getByLabel('Server Name').fill(serverName);

    // Submit the form via the "Create Server" button
    await page.getByRole('button', { name: 'Create Server' }).click();

    // Verify navigation to server detail page or success toast
    await expect(
      page.getByText('created successfully').or(page.locator(`text=${serverName}`)),
    ).toBeVisible({ timeout: 15_000 });
  });

  test('server appears in the server list', async ({ userPage: page }) => {
    await page.goto('/servers');

    await expect(
      page.getByRole('heading', { name: 'My Servers' }),
    ).toBeVisible();
    await expect(page.getByText(serverName)).toBeVisible({ timeout: 10_000 });
  });

  test('user can delete a game server', async ({ userPage: page }) => {
    await page.goto('/servers');

    // Wait for the server card to be visible
    await expect(page.getByText(serverName)).toBeVisible({ timeout: 10_000 });

    // Find the card containing the server name and click its delete (Trash2) button.
    // The trash button is the last button in the card footer, styled with text-destructive.
    const serverCard = page
      .locator('[class*="card"]')
      .filter({ hasText: serverName });
    await serverCard
      .locator('button.text-destructive, button:has(.text-destructive)')
      .first()
      .click();

    // Verify server is removed from the list (React Query auto-refreshes within 5s)
    await expect(page.getByText(serverName)).not.toBeVisible({
      timeout: 15_000,
    });
  });
});
