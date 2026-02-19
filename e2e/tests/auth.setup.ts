import { test as setup, expect } from '@playwright/test';
import { execSync } from 'child_process';
import fs from 'fs';
import path from 'path';

const ADMIN_USERNAME = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-test-password';
const ADMIN_EMAIL = 'admin@e2e-test.local';
const NAMESPACE = 'kterodactyl-system';

const authDir = path.join(__dirname, '../playwright/.auth');
const adminAuthFile = path.join(authDir, 'admin.json');
const userAuthFile = path.join(authDir, 'user.json');

// Resolve project root (parent of e2e/)
const projectRoot = path.resolve(__dirname, '../..');

setup('seed admin user via kubectl', async () => {
  fs.mkdirSync(authDir, { recursive: true });

  // Generate the Argon2id password hash using the project's auth package
  const passwordHash = execSync(
    `go run ./hack/hash-password.go "${ADMIN_PASSWORD}"`,
    { cwd: projectRoot, encoding: 'utf-8' },
  ).trim();

  // Create the admin user Secret (idempotent via dry-run + apply)
  execSync(
    `kubectl create secret generic user-${ADMIN_USERNAME} ` +
      `--namespace=${NAMESPACE} ` +
      `--from-literal=email=${ADMIN_EMAIL} ` +
      `--from-literal=password-hash='${passwordHash}' ` +
      `--from-literal=created-at=2026-01-01T00:00:00Z ` +
      `--from-literal=invited-by=bootstrap ` +
      `--dry-run=client -o yaml | kubectl apply -f -`,
    { cwd: projectRoot, stdio: 'inherit' },
  );

  // Label the Secret so the UserStore can find it
  execSync(
    `kubectl label secret user-${ADMIN_USERNAME} ` +
      `--namespace=${NAMESPACE} --overwrite ` +
      `kterodactyl.io/resource-type=user ` +
      `kterodactyl.io/user=${ADMIN_USERNAME} ` +
      `kterodactyl.io/role=admin ` +
      `kterodactyl.io/managed-by=kterodactyl`,
    { cwd: projectRoot, stdio: 'inherit' },
  );
});

setup('authenticate as admin', async ({ request }) => {
  const response = await request.post('/api/v1/auth/login', {
    data: {
      username: ADMIN_USERNAME,
      password: ADMIN_PASSWORD,
    },
  });
  expect(response.ok()).toBeTruthy();
  const { token } = await response.json();

  fs.writeFileSync(adminAuthFile, JSON.stringify({ token }));
});

setup('create regular user and authenticate', async ({ request }) => {
  // Read admin token
  const adminAuth = JSON.parse(fs.readFileSync(adminAuthFile, 'utf-8'));

  // Create invite via admin API
  const inviteResponse = await request.post('/api/v1/admin/invites', {
    headers: { Authorization: `Bearer ${adminAuth.token}` },
    data: { email: 'e2e-user@test.local' },
  });
  expect(inviteResponse.ok()).toBeTruthy();
  const { token: inviteToken } = await inviteResponse.json();

  // Register regular user
  const registerResponse = await request.post('/api/v1/auth/register', {
    data: {
      username: 'e2e-user',
      email: 'e2e-user@test.local',
      password: ADMIN_PASSWORD,
      inviteToken,
    },
  });
  expect(registerResponse.ok()).toBeTruthy();
  const { token } = await registerResponse.json();

  fs.writeFileSync(userAuthFile, JSON.stringify({ token }));
});
