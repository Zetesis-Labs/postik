import { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, createRoute, createRouter, Outlet, redirect } from '@tanstack/react-router';
import { meQuery } from '@/lib/api';
import { ModalManager } from '@/components/ui/modals';
import { LoginPage } from '@/components/auth/login-page';
import { SuperadminLoginPage } from '@/components/auth/superadmin-login';
import { AdminPanel } from '@/components/admin/admin-panel';

type RouterContext = { queryClient: QueryClient };

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <ModalManager>
      <Outlet />
    </ModalManager>
  ),
  notFoundComponent: () => {
    throw redirect({ to: '/' });
  },
});

const homeFor = (kind: string | undefined) => (kind === 'superadmin' ? '/admin' : '/auth/login');

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.ensureQueryData(meQuery);
    throw redirect({ to: homeFor(me?.kind) });
  },
});

const redirectIfSignedIn = async ({ context }: { context: RouterContext }) => {
  const me = await context.queryClient.ensureQueryData(meQuery);
  if (me) {
    throw redirect({ to: homeFor(me.kind) });
  }
};

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/auth/login',
  beforeLoad: redirectIfSignedIn,
  component: LoginPage,
});

const superadminLoginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/auth/admin',
  beforeLoad: redirectIfSignedIn,
  component: SuperadminLoginPage,
});

const adminRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/admin',
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.ensureQueryData(meQuery);
    if (me?.kind !== 'superadmin') {
      throw redirect({ to: '/auth/login' });
    }
  },
  component: AdminPanel,
});

const routeTree = rootRoute.addChildren([indexRoute, loginRoute, superadminLoginRoute, adminRoute]);

export function buildRouter(queryClient: QueryClient) {
  return createRouter({ routeTree, context: { queryClient }, defaultPreload: 'intent' });
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof buildRouter>;
  }
}
