import { execFileSync } from 'node:child_process';
import { createHmac } from 'node:crypto';

export const superadmin = {
  username: 'admin',
  password: 'e2e-password',
  totpSecret: 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP',
};

function base32Decode(input: string): Buffer {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const char of input.replace(/=+$/, '').toUpperCase()) {
    bits += alphabet.indexOf(char).toString(2).padStart(5, '0');
  }
  const bytes = [];
  for (let i = 0; i + 8 <= bits.length; i += 8) {
    bytes.push(parseInt(bits.slice(i, i + 8), 2));
  }
  return Buffer.from(bytes);
}

export function totp(secret: string, now = Date.now()): string {
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(now / 30_000)));
  const digest = createHmac('sha1', base32Decode(secret)).update(counter).digest();
  const offset = digest[digest.length - 1] & 0x0f;
  const value = (digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000;
  return value.toString().padStart(6, '0');
}

// psql runs a statement against the throwaway database that e2e/serve.sh creates.
export function psql(sql: string): string {
  const admin = process.env.TEST_DATABASE_URL;
  if (!admin) {
    throw new Error('TEST_DATABASE_URL is required');
  }
  const url = new URL(admin);
  url.pathname = '/postik_e2e';
  return execFileSync('psql', [url.toString(), '-tA', '-v', 'ON_ERROR_STOP=1', '-c', sql], { encoding: 'utf8' }).trim();
}

export function uniqueSubject(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
}
