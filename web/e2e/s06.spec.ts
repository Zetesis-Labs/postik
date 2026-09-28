import { expect, Page, test } from '@playwright/test';
import { fakesURL, psql, signIn, uniqueSubject } from './support';

// allowLinkedIn fills the form of the fake LinkedIn and comes back to postik.
async function allowLinkedIn(page: Page, subject: string, name: string, pages = '') {
  await expect(page).toHaveURL(/\/linkedin\/oauth\/v2\/authorization/);
  await page.locator('input[name="sub"]').fill(subject);
  await page.locator('input[name="name"]').fill(name);
  await page.locator('input[name="pages"]').fill(pages);
  await page.getByRole('button', { name: 'Allow' }).click();
  await expect(page).toHaveURL(/\/launches$/);
}

async function connectLinkedIn(page: Page, name: string) {
  const subject = uniqueSubject('linkedin');
  await page.getByRole('button', { name: 'Agregar canal' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'LinkedIn', exact: true }).click();
  await allowLinkedIn(page, subject, name);
  const channel = page.getByTestId('channel').filter({ hasText: name });
  await expect(channel).toBeVisible();
  return { subject, channel };
}

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

test('S06.18 Conectar LinkedIn desde la rejilla', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Lucía');

  const { channel } = await connectLinkedIn(page, 'Lucía en LinkedIn');

  await expect(page.getByText('Canal agregado')).toBeVisible();
  await expect(channel.locator('img').first()).toHaveAttribute('src', /^\/uploads\/avatars\//);
  await context.close();
});

test('S06.19 Conectar LinkedIn Page elige la página en la rejilla', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Marta');
  const suffix = uniqueSubject('p');
  const zetesis = `Zetesis ${suffix}`;

  await page.getByRole('button', { name: 'Agregar canal' }).click();
  await page.getByRole('dialog').getByRole('button', { name: 'LinkedIn Page' }).click();
  await allowLinkedIn(page, uniqueSubject('marta'), 'Marta', `${zetesis}, Nexo ${suffix}`);

  const grid = page.getByTestId('continue-page');
  await expect(grid.getByRole('option')).toHaveCount(2);
  await grid.getByRole('option', { name: zetesis }).click();
  await grid.getByRole('button', { name: 'Guardar' }).click();

  await expect(grid).toHaveCount(0);
  const channel = page.getByTestId('channel').filter({ hasText: zetesis });
  await expect(channel).toBeVisible();
  await expect(channel.getByTestId('channel-attention')).toHaveCount(0);
  await context.close();
});

test('S06.20 Un canal desconectado se ve y se reconecta', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Nuria');
  const name = `Nuria ${uniqueSubject('li')}`;
  const { subject } = await connectLinkedIn(page, name);
  psql(`UPDATE channels SET refresh_needed = true WHERE name = '${name}'`);
  await page.reload();

  const channel = page.getByTestId('channel').filter({ hasText: name });
  await expect(channel).toHaveAttribute('data-refresh-needed', 'true');
  await channel.hover();
  await expect(page.getByText('Canal desconectado, haz clic para reconectar.')).toBeVisible();
  await channel.getByTestId('channel-attention').click();
  await allowLinkedIn(page, subject, name);

  await expect(channel).toHaveAttribute('data-refresh-needed', 'false');
  await expect(channel.getByTestId('channel-attention')).toHaveCount(0);
  await context.close();
});

test('S06.21 «Publicar ya» sale en LinkedIn y la tarjeta enlaza a la publicación', async ({ browser, request }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Olga');
  await connectLinkedIn(page, 'Olga en LinkedIn');
  const text = `Sale en LinkedIn ${uniqueSubject('post')}`;

  await publishNow(page, text);

  const card = page.getByTestId('calendar-post').filter({ hasText: text });
  await expect(card.getByTestId('preview-post')).toHaveAttribute('href', /^https:\/\/www\.linkedin\.com\/feed\/update\/urn:li:share:\d+\/$/, {
    timeout: 20_000,
  });
  const posts = await (await request.get(`${fakesURL}/linkedin/_control/posts`)).json();
  expect(posts.some((p: { commentary: string }) => p.commentary.includes('Sale en LinkedIn'))).toBe(true);
  await context.close();
});
