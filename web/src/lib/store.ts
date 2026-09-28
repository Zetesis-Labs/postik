// A minimal store with the shape of zustand's `create`, built on
// useSyncExternalStore, so screen state ported from Postiz keeps its API.
import { useRef, useSyncExternalStore } from 'react';

type SetState<T> = (partial: Partial<T> | ((state: T) => Partial<T>)) => void;

export type UseStore<T> = {
  <U>(selector: (state: T) => U): U;
  getState: () => T;
  setState: SetState<T>;
};

export function create<T>(init: (set: SetState<T>, get: () => T) => T): UseStore<T> {
  let state: T;
  const listeners = new Set<() => void>();
  const set: SetState<T> = (partial) => {
    const next = typeof partial === 'function' ? partial(state) : partial;
    state = { ...state, ...next };
    listeners.forEach((listener) => listener());
  };
  const get = () => state;
  state = init(set, get);
  const subscribe = (listener: () => void) => {
    listeners.add(listener);
    return () => listeners.delete(listener);
  };
  const useStore = (<U>(selector: (state: T) => U) =>
    useSyncExternalStore(subscribe, () => selector(state))) as UseStore<T>;
  useStore.getState = get;
  useStore.setState = set;
  return useStore;
}

function shallowEqual<U>(a: U, b: U): boolean {
  if (Object.is(a, b)) {
    return true;
  }
  if (typeof a !== 'object' || typeof b !== 'object' || a === null || b === null) {
    return false;
  }
  const keysA = Object.keys(a);
  if (keysA.length !== Object.keys(b).length) {
    return false;
  }
  return keysA.every((key) => Object.is((a as Record<string, unknown>)[key], (b as Record<string, unknown>)[key]));
}

// useShallow keeps the previous object while its fields are the same, like
// zustand's, so selectors that build objects do not re-render forever.
export function useShallow<T, U>(selector: (state: T) => U): (state: T) => U {
  const previous = useRef<U | undefined>(undefined);
  return (state: T) => {
    const next = selector(state);
    if (previous.current !== undefined && shallowEqual(previous.current, next)) {
      return previous.current;
    }
    previous.current = next;
    return next;
  };
}
