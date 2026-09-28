import { expect, Page, test } from '@playwright/test';
import { psql, uniqueSubject } from './support';

async function signInWithFake(page: Page, subject: string, name: string) {
  await page.goto('/auth/login');
  await page.getByRole('button', { name: /Iniciar sesión con\s+Fake/ }).click();
  await page.locator('input[name="sub"]').fill(subject);
  await page.locator('input[name="email"]').fill(`${subject}@example.com`);
  await page.locator('input[name="name"]').fill(name);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page).toHaveURL(/\/launches$/);
}

test('S02.11 una persona entra con el proveedor, llega al calendario y sale', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();

  await signInWithFake(page, uniqueSubject('lucia'), 'Lucía');
  await expect(page.getByText('Calendario', { exact: true }).first()).toBeVisible();
  await expect(page.getByTestId('organization-selector')).toHaveCount(0);

  await page.getByRole('button', { name: 'Cerrar sesión' }).click();
  await page.getByRole('button', { name: 'Sí, cerrar sesión' }).click();
  await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
  await context.close();
});

test('S02.12 con dos organizaciones aparece el selector y cambia la activa', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  const subject = uniqueSubject('marta');

  await signInWithFake(page, subject, 'Marta');
  const orgID = crypto.randomUUID();
  psql(
    `INSERT INTO organizations (id, name, created_at) VALUES ('${orgID}', 'Agencia Norte', now() + interval '1 hour');` +
      `INSERT INTO memberships (organization_id, user_id, role, created_at) ` +
      `SELECT '${orgID}', id, 'ADMIN', now() + interval '1 hour' FROM users WHERE subject = '${subject}';`
  );

  await page.reload();
  const selector = page.getByTestId('organization-selector');
  await expect(page.getByTestId('current-organization')).toHaveText('Marta');
  await selector.hover();
  await page.getByRole('button', { name: /Agencia Norte/ }).click();
  await expect(page.getByTestId('current-organization')).toHaveText('Agencia Norte');
  await context.close();
});
