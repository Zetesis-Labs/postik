import { expect, Page, test } from '@playwright/test';
import { connectTelegram, fakesURL, signIn } from './support';

async function publishNow(page: Page, text: string) {
  await page.getByTestId('create-post').click();
  const editor = page.getByTestId('post-editor');
  await editor.getByTestId('pick-channel').first().click();
  const input = editor.locator('.ProseMirror').first();
  await input.click();
  await page.keyboard.type(text);
  await editor.getByRole('button', { name: 'Agregar al calendario' }).hover();
  await editor.getByRole('button', { name: 'Publicar ahora' }).click();
  await expect(editor).toHaveCount(0);
}

test('S05.15 «Publicar ya» sale en Telegram y la tarjeta enlaza a la publicación', async ({ browser, request }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Sofía');
  await connectTelegram(page, request, 'Grupo Publicar');

  await publishNow(page, 'Sale ahora mismo');

  const card = page.getByTestId('calendar-post').filter({ hasText: 'Sale ahora mismo' });
  await expect(card.getByTestId('preview-post')).toHaveAttribute('href', /^https:\/\/t\.me\/c\/\d+\/\d+$/, { timeout: 20_000 });
  const sent = await (await request.get(`${fakesURL}/telegram/_control/sent`)).json();
  expect(sent.some((m: { text: string }) => m.text === 'Sale ahora mismo')).toBe(true);
  await context.close();
});

test('S05.16 Un post en Error se ve en rojo con su motivo', async ({ browser, request }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Tomás');
  await connectTelegram(page, request, 'Grupo Rechazo');
  const prepared = await request.post(`${fakesURL}/telegram/_control/fail`, {
    data: { code: 400, description: 'Bad Request: chat not found' },
  });
  expect(prepared.status()).toBe(204);

  await publishNow(page, 'Esto no sale');

  const card = page.getByTestId('calendar-post').filter({ hasText: 'Esto no sale' });
  const badge = card.getByTestId('post-error');
  await expect(badge).toBeVisible({ timeout: 20_000 });
  await badge.hover();
  await expect(page.getByText('Bad Request: chat not found')).toBeVisible();
  await context.close();
});
