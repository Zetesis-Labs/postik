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
