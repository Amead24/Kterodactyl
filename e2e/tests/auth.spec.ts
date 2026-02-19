import fs from 'fs';
import path from 'path';
import { test, expect } from '../fixtures/auth';

test.describe('Authentication', () => {
  test('authenticated user sees the dashboard', async ({ userPage: page }) => {
    await page.goto('/');

    await expect(
      page.getByRole('heading', { name: 'Dashboard' }),
    ).toBeVisible();
    await expect(page.getByText('Welcome back')).toBeVisible();
  });

  test('new user can sign up with invite token and see dashboard', async ({
    page,
  }) => {
    // Read admin token from the setup-generated auth file
    const adminAuthFile = path.resolve(
      __dirname,
      '../playwright/.auth/admin.json',
    );
    const { token: adminToken } = JSON.parse(
      fs.readFileSync(adminAuthFile, 'utf-8'),
    );

    // Create an invite via the admin API
    const inviteResponse = await page.request.post('/api/v1/admin/invites', {
      headers: { Authorization: `Bearer ${adminToken}` },
      data: { email: 'e2e-signup-test@test.local' },
    });
    expect(inviteResponse.ok()).toBeTruthy();
    const { token: inviteToken } = await inviteResponse.json();

    // Navigate to the registration page
    await page.goto('/register');

    // Fill the registration form (labels match register.tsx)
    const uniqueUsername = `e2e-signup-${Date.now()}`;
    await page.getByLabel('Username').fill(uniqueUsername);
    await page.getByLabel('Email').fill('e2e-signup-test@test.local');
    await page.getByLabel('Password').fill('e2e-test-password');
    await page.getByLabel('Invite Token').fill(inviteToken);

    // Submit the registration form
    await page.getByRole('button', { name: 'Create account' }).click();

    // After successful registration, user is auto-logged in and redirected to /
    await expect(
      page.getByRole('heading', { name: 'Dashboard' }),
    ).toBeVisible();
    await expect(page).toHaveURL('/');
  });
});
