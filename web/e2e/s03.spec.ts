import { test, expect } from '@playwright/test';
import { connectTelegram, signIn } from './support';

test('S03.12 añadir un canal de Telegram desde la rejilla hasta verlo en la barra lateral', async ({ browser, request }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Nora');

  await expect(page.getByText('Aún no hay canales')).toBeVisible();
  await connectTelegram(page, request, 'Grupo E2E');
  await expect(page.getByText('Aún no hay canales')).toHaveCount(0);
  await context.close();
});

test('S03.13 el menú contextual mueve a cliente, edita franjas, desactiva y borra', async ({ browser, request }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Olga');
  const channel = await connectTelegram(page, request, 'Grupo Menú');

  await channel.getByTestId('channel-menu').click();
  await page.getByRole('menuitem', { name: 'Mover / agregar al grupo' }).click();
  await page.locator('#customer-name').fill('Acme');
  await page.getByRole('button', { name: 'Guardar' }).click();
  await expect(page.getByTestId('channel-group-Acme').getByTestId('channel').filter({ hasText: 'Grupo Menú' })).toBeVisible();

  await page.getByTestId('channel').filter({ hasText: 'Grupo Menú' }).getByTestId('channel-menu').click();
  await page.getByRole('menuitem', { name: 'Editar franjas horarias' }).click();
  await expect(page.getByTestId('time-slots').locator('> div')).toHaveCount(3);
  await page.getByRole('button', { name: 'Agregar', exact: true }).click();
  await expect(page.getByTestId('time-slots').locator('> div')).toHaveCount(4);
  await page.getByRole('button', { name: 'Guardar cambios' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);

  await page.getByTestId('channel').filter({ hasText: 'Grupo Menú' }).getByTestId('channel-menu').click();
  await page.getByRole('menuitem', { name: 'Editar franjas horarias' }).click();
  await expect(page.getByTestId('time-slots').locator('> div')).toHaveCount(4);
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'Sí' }).click();

  await page.getByTestId('channel').filter({ hasText: 'Grupo Menú' }).getByTestId('channel-menu').click();
  await page.getByRole('menuitem', { name: 'Deshabilitar canal' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Deshabilitar canal' }).click();
  await expect(page.getByTestId('channel').filter({ hasText: 'Grupo Menú' })).toHaveAttribute('data-disabled', 'true');

  await page.getByTestId('channel').filter({ hasText: 'Grupo Menú' }).getByTestId('channel-menu').click();
  await page.getByRole('menuitem', { name: 'Eliminar' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'Eliminar canal' }).click();
  await expect(page.getByTestId('channel').filter({ hasText: 'Grupo Menú' })).toHaveCount(0);
  await context.close();
});
