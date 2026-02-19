import { test as base, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';

/**
 * Custom auth fixtures that provide pre-authenticated browser contexts for
 * admin and regular user roles. Each fixture reads the JWT token written by
 * the setup project and injects it via addInitScript so the Zustand auth
 * store is populated before any page JavaScript runs.
 */

type AuthFixtures = {
  adminPage: Awaited<ReturnType<typeof createAuthPage>>;
  userPage: Awaited<ReturnType<typeof createAuthPage>>;
};

async function createAuthPage(
  browser: Parameters<Parameters<typeof base.extend>[0]['adminPage']>[0]['browser'],
  authFile: string,
  use: (page: any) => Promise<void>,
) {
  const { token } = JSON.parse(fs.readFileSync(authFile, 'utf-8'));

  const context = await browser.newContext();
  await context.addInitScript((t: string) => {
    (window as any).__KTERODACTYL_E2E_TOKEN = t;
  }, token);

  const page = await context.newPage();
  await use(page);
  await context.close();
}

export const test = base.extend<AuthFixtures>({
  adminPage: async ({ browser }, use) => {
    const authFile = path.join(__dirname, '../playwright/.auth/admin.json');
    await createAuthPage(browser, authFile, use);
  },

  userPage: async ({ browser }, use) => {
    const authFile = path.join(__dirname, '../playwright/.auth/user.json');
    await createAuthPage(browser, authFile, use);
  },
});

export { expect } from '@playwright/test';
