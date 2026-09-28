import { expect, test } from '@playwright/test';
import { superadmin, totp } from './support';

test('S01.13 la pantalla de acceso sale en el idioma del navegador y el selector lo cambia', async ({ browser }) => {
  const spanish = await browser.newContext({ locale: 'es-ES' });
  const spanishPage = await spanish.newPage();
  await spanishPage.goto('/auth/login');
  await expect(spanishPage.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
  await spanish.close();

  const english = await browser.newContext({ locale: 'en-US' });
  const page = await english.newPage();
  await page.goto('/auth/login');
  await expect(page.getByRole('heading', { name: 'Sign In' })).toBeVisible();

  await page.getByRole('button', { name: 'Change Language' }).click();
  await page.locator('[data-language="es"]').click();
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
  await english.close();
});

test('S01.14 el superadmin entra con TOTP, llega a su panel y sale', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();

  await page.goto('/auth/login');
  await page.getByRole('link', { name: 'Acceso de administrador' }).click();
  await page.locator('input[name="username"]').fill(superadmin.username);
  await page.locator('input[name="password"]').fill(superadmin.password);
  await page.locator('input[name="code"]').fill(totp(superadmin.totpSecret));
  await page.getByRole('button', { name: 'Entrar' }).click();

  await expect(page.getByText('Panel del superadmin', { exact: true })).toBeVisible();
  await expect(page).toHaveURL(/\/admin$/);

  await page.getByRole('button', { name: 'Cerrar sesión' }).click();
  await page.getByRole('button', { name: 'Sí, cerrar sesión' }).click();
  await expect(page.getByRole('heading', { name: 'Iniciar sesión' })).toBeVisible();
  await context.close();
});
