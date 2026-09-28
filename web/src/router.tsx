import { QueryClient } from '@tanstack/react-query';
import { createRootRouteWithContext, createRoute, createRouter, Outlet, redirect } from '@tanstack/react-router';
import { meQuery } from '@/lib/api';
import { ModalManager } from '@/components/ui/modals';
import { Toaster } from '@/components/ui/toaster';
import { ToolTip } from '@/components/ui/tooltip';
import { LoginPage } from '@/components/auth/login-page';
import { SuperadminLoginPage } from '@/components/auth/superadmin-login';
import { AdminPanel } from '@/components/admin/admin-panel';
import { LaunchesPage } from '@/components/launches/launches-page';
import { MediaPage } from '@/components/media/media-page';

type RouterContext = { queryClient: QueryClient };

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: () => (
    <ModalManager>
      <ToolTip />
      <Toaster />
      <Outlet />
    </ModalManager>
  ),
  notFoundComponent: () => {
    throw redirect({ to: '/' });
  },
});

const homeFor = (kind: string | undefined) => {
  switch (kind) {
    case 'superadmin':
      return '/admin';
    case 'member':
      return '/launches';
    default:
      return '/auth/login';
  }
};

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
  validateSearch: (search: Record<string, unknown>): { error?: string } =>
    typeof search.error === 'string' ? { error: search.error } : {},
  beforeLoad: redirectIfSignedIn,
  component: function Login() {
    const { error } = loginRoute.useSearch();
    return <LoginPage error={error} />;
  },
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

const launchesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/launches',
  beforeLoad: async ({ context }) => {
    const me = await context.queryClient.ensureQueryData(meQuery);
    if (me?.kind !== 'member') {
      throw redirect({ to: homeFor(me?.kind) });
    }
  },
  component: LaunchesPage,
});

const requireMember = async ({ context }: { context: RouterContext }) => {
  const me = await context.queryClient.ensureQueryData(meQuery);
  if (me?.kind !== 'member') {
    throw redirect({ to: homeFor(me?.kind) });
  }
};

const mediaRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/media',
  beforeLoad: requireMember,
  component: MediaPage,
});

const routeTree = rootRoute.addChildren([indexRoute, loginRoute, superadminLoginRoute, adminRoute, launchesRoute, mediaRoute]);

export function buildRouter(queryClient: QueryClient) {
  return createRouter({ routeTree, context: { queryClient }, defaultPreload: 'intent' });
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof buildRouter>;
  }
}
