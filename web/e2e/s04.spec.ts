import { expect, Page, test } from '@playwright/test';
import { connectTelegram, signIn } from './support';

const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==', 'base64');

function isoDate(date: Date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

// weekOfTomorrow opens the week view on the week that holds tomorrow, so the
// slot is in the future and on screen whatever day the suite runs.
async function weekOfTomorrow(page: Page) {
  const tomorrow = new Date();
  tomorrow.setDate(tomorrow.getDate() + 1);
  const monday = new Date(tomorrow);
  monday.setDate(tomorrow.getDate() - ((tomorrow.getDay() + 6) % 7));
  const sunday = new Date(monday);
  sunday.setDate(monday.getDate() + 6);
  await page.goto(`/launches?startDate=${isoDate(monday)}&endDate=${isoDate(sunday)}&display=week`);
  return page.getByTestId(`slot-${isoDate(tomorrow)}-10`);
}

async function writeInEditor(page: Page, text: string) {
  const editor = page.getByTestId('post-editor').locator('.ProseMirror').first();
  await editor.click();
  await page.keyboard.press('ControlOrMeta+a');
  await page.keyboard.type(text);
}

test('S04.21 crear un post desde un hueco del calendario y verlo en la semana', async ({ browser, request }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Pilar');
  await connectTelegram(page, request, 'Grupo Posts');

  const slot = await weekOfTomorrow(page);
  await slot.getByTestId('add-post-slot').click();
  const editor = page.getByTestId('post-editor');
  await editor.getByTestId('pick-channel').first().click();
  await writeInEditor(page, 'Primer post desde postik');
  await expect(page.getByTestId('post-preview')).toContainText('Primer post desde postik');
  await editor.getByRole('button', { name: 'Agregar al calendario' }).click();

  await expect(editor).toHaveCount(0);
  await expect(slot.getByTestId('calendar-post')).toContainText('Primer post desde postik');
  await context.close();
});

test('S04.22 abrir un post, actualizarlo y borrarlo', async ({ browser, request }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Quique');
  await connectTelegram(page, request, 'Grupo Edición');

  const slot = await weekOfTomorrow(page);
  await slot.getByTestId('add-post-slot').click();
  await page.getByTestId('post-editor').getByTestId('pick-channel').first().click();
  await writeInEditor(page, 'Texto original');
  await page.getByTestId('post-editor').getByRole('button', { name: 'Agregar al calendario' }).click();
  await expect(slot.getByTestId('calendar-post')).toContainText('Texto original');

  await slot.getByTestId('calendar-post').click();
  const editor = page.getByTestId('post-editor');
  await expect(editor.locator('.ProseMirror').first()).toContainText('Texto original');
  await writeInEditor(page, 'Texto corregido');
  await editor.getByRole('button', { name: 'Actualizar', exact: true }).click();
  await expect(editor).toHaveCount(0);
  await expect(slot.getByTestId('calendar-post')).toContainText('Texto corregido');

  await slot.getByTestId('calendar-post').click();
  await page.getByTestId('post-editor').getByRole('button', { name: 'Eliminar publicación' }).click();
  await page.getByRole('dialog').getByRole('button', { name: '¡Sí, bórralo!' }).click();
  await expect(page.getByTestId('post-editor')).toHaveCount(0);
  await expect(slot.getByTestId('calendar-post')).toHaveCount(0);
  await context.close();
});

test('S04.23 subir un medio en la biblioteca e insertarlo en un post', async ({ browser, request }) => {
  const context = await browser.newContext({ locale: 'es-ES' });
  const page = await context.newPage();
  await signIn(page, 'Rosa');
  await connectTelegram(page, request, 'Grupo Medios');

  await page.goto('/media');
  await page.getByTestId('media-upload-input').setInputFiles({ name: 'pixel.png', mimeType: 'image/png', buffer: png });
  await expect(page.getByTestId('media-item')).toHaveCount(1);

  await page.goto('/launches');
  await page.getByTestId('create-post').click();
  const editor = page.getByTestId('post-editor');
  await editor.getByTestId('insert-media').first().click();
  await page.getByTestId('media-item').click({ position: { x: 12, y: 12 } });
  await page.getByRole('button', { name: 'Agregar medios seleccionados' }).click();

  await expect(editor.getByTestId('post-media')).toHaveCount(1);
  await expect(editor.getByTestId('post-media').locator('img')).toHaveAttribute('src', /\/uploads\//);
  await context.close();
});
