import createClient from 'openapi-fetch';
import { queryOptions } from '@tanstack/react-query';
import type { components, paths } from './api-schema';

export type Me = components['schemas']['Me'];
export type Instance = components['schemas']['Instance'];
export type ApiError = components['schemas']['Error'];

export const api = createClient<paths>({ baseUrl: '/api/v1', credentials: 'same-origin' });

export const meQuery = queryOptions({
  queryKey: ['me'],
  queryFn: async (): Promise<Me | null> => {
    const { data, response } = await api.GET('/me');
    if (response.status === 401) {
      return null;
    }
    if (!data) {
      throw new Error(`GET /me answered ${response.status}`);
    }
    return data;
  },
  staleTime: 60_000,
});

export const instanceQuery = queryOptions({
  queryKey: ['instance'],
  queryFn: async (): Promise<Instance> => {
    const { data, response } = await api.GET('/instance');
    if (!data) {
      throw new Error(`GET /instance answered ${response.status}`);
    }
    return data;
  },
  staleTime: Infinity,
});

export type Channel = components['schemas']['Channel'];
export type Provider = components['schemas']['Provider'];
export type Customer = components['schemas']['Customer'];

async function required<T>(call: Promise<{ data?: T; response: Response }>, what: string): Promise<T> {
  const { data, response } = await call;
  if (data === undefined) {
    throw new Error(`${what} answered ${response.status}`);
  }
  return data;
}

export const channelsQuery = queryOptions({
  queryKey: ['channels'],
  queryFn: () => required(api.GET('/channels'), 'GET /channels'),
});

export const providersQuery = queryOptions({
  queryKey: ['providers'],
  queryFn: () => required(api.GET('/channels/providers'), 'GET /channels/providers'),
  staleTime: Infinity,
});

export const customersQuery = queryOptions({
  queryKey: ['customers'],
  queryFn: () => required(api.GET('/customers'), 'GET /customers'),
});
