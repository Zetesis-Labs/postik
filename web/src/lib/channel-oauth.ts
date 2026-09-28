import { api } from '@/lib/api';

// startAuthorization sends the browser to the network to connect a channel,
// or to reconnect channelId. The network sends it back to /launches.
export async function startAuthorization(provider: string, channelId?: string): Promise<void> {
  const { data, response } = await api.POST('/channels/{provider}/authorizations', {
    params: { path: { provider } },
    body: channelId ? { channelId } : {},
  });
  if (!data) {
    throw new Error(`POST /channels/${provider}/authorizations answered ${response.status}`);
  }
  window.location.assign(data.url);
}

// providerNames are the names of the networks as Postiz shows them.
export const providerNames: Record<string, string> = {
  telegram: 'Telegram',
  linkedin: 'LinkedIn',
  'linkedin-page': 'LinkedIn Page',
  x: 'X',
};

export function providerName(identifier: string): string {
  return providerNames[identifier] ?? (identifier ? identifier[0].toUpperCase() + identifier.slice(1) : '');
}
