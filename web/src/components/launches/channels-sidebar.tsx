// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/launches.component.tsx (AGPL-3.0).
import { FC, useCallback, useMemo, useState } from 'react';
import clsx from 'clsx';
import { useDrag, useDrop } from 'react-dnd';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, Channel, channelsQuery, customersQuery } from '@/lib/api';
import { useT } from '@/lib/i18n';
import { AddProviderButton } from '@/components/launches/add-provider';
import { Menu } from '@/components/launches/channel-menu';
import { NewPost } from '@/components/launches/new-post';
import { storedMode } from '@/components/layout/mode';

export const SVGLine = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="5" height="52" viewBox="0 0 5 52" fill="none" className="rtl:rotate-180">
    <path d="M0.5 4C0.5 1.79086 2.29086 0 4.5 0V52C2.29086 52 0.5 50.2091 0.5 48V4Z" fill="url(#paint0_linear_1930_1119)" />
    <path d="M0.5 4C0.5 1.79086 2.29086 0 4.5 0V52C2.29086 52 0.5 50.2091 0.5 48V4Z" fill="url(#paint1_radial_1930_1119)" />
    <defs>
      <linearGradient id="paint0_linear_1930_1119" x1="-7" y1="-27.7727" x2="-2.58929" y2="-28.6843" gradientUnits="userSpaceOnUse">
        <stop stopColor="#662FDA" />
        <stop offset="1" stopColor="#5720CB" />
      </linearGradient>
      <radialGradient
        id="paint1_radial_1930_1119"
        cx="0"
        cy="0"
        r="1"
        gradientUnits="userSpaceOnUse"
        gradientTransform="translate(1.19333 7.45342) rotate(21.2064) scale(16.1503 188.627)"
      >
        <stop stopColor="#8C66FF" />
        <stop offset="1" stopColor="#8C66FF" stopOpacity="0" />
      </radialGradient>
    </defs>
  </svg>
);

export const OpenClose: FC<{ isOpen: boolean }> = ({ isOpen }) => (
  <svg
    width="11"
    height="6"
    viewBox="0 0 22 12"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={clsx('rotate-180 transition-all', isOpen ? 'rotate-180' : 'rotate-90')}
  >
    <path
      d="M21.9245 11.3823C21.8489 11.5651 21.7207 11.7213 21.5563 11.8312C21.3919 11.9411 21.1986 11.9998 21.0008 11.9998H1.00079C0.802892 12 0.609399 11.9414 0.444805 11.8315C0.280212 11.7217 0.151917 11.5654 0.076165 11.3826C0.000412494 11.1998 -0.0193921 10.9986 0.0192583 10.8045C0.0579087 10.6104 0.153276 10.4322 0.293288 10.2923L10.2933 0.29231C10.3862 0.199333 10.4964 0.125575 10.6178 0.0752506C10.7392 0.0249263 10.8694 -0.000976562 11.0008 -0.000976562C11.1322 -0.000976562 11.2623 0.0249263 11.3837 0.0752506C11.5051 0.125575 11.6154 0.199333 11.7083 0.29231L21.7083 10.2923C21.8481 10.4322 21.9433 10.6105 21.9818 10.8045C22.0202 10.9985 22.0003 11.1996 21.9245 11.3823Z"
      fill="currentColor"
    />
  </svg>
);

function readFlag(key: string, fallback: string): string {
  try {
    return window.localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

function writeFlag(key: string, value: string) {
  try {
    window.localStorage.setItem(key, value);
  } catch {
    // Without storage the choice lasts until the page reloads.
  }
}

type Group = { id: string; name: string; values: Channel[] };

const MenuGroupComponent: FC<{
  group: Group;
  collapsed: boolean;
  changeItemGroup: (id: string, group: string) => void;
  onChange: () => void;
}> = ({ group, collapsed, changeItemGroup, onChange }) => {
  const [isOpen, setIsOpen] = useState(() => !!+readFlag(group.name + '_isOpen', '1'));
  const changeOpenClose = useCallback(() => {
    setIsOpen(!isOpen);
    writeFlag(group.name + '_isOpen', isOpen ? '0' : '1');
  }, [group.name, isOpen]);
  const [collectedProps, drop] = useDrop(
    () => ({
      accept: 'menu',
      drop: (item: { id: string }) => changeItemGroup(item.id, group.id),
      collect: (monitor) => ({ isOver: !!monitor.isOver() }),
    }),
    [changeItemGroup, group.id]
  );
  return (
    <div className="gap-[16px] flex flex-col relative" ref={(node) => void drop(node)} data-testid={`channel-group-${group.name || 'none'}`}>
      {collectedProps.isOver && (
        <div className="absolute start-0 top-0 w-full h-full pointer-events-none">
          <div className="w-full h-full start-0 top-0 relative">
            <div className="bg-white/30 w-full h-full p-[8px] box-content rounded-md" />
          </div>
        </div>
      )}
      {!!group.name && (
        <div className="flex items-center gap-[5px] cursor-pointer" onClick={changeOpenClose}>
          <div>
            <OpenClose isOpen={isOpen} />
          </div>
          <div
            className="line-clamp-1"
            {...(collapsed ? { 'data-tooltip-id': 'tooltip', 'data-tooltip-content': group.name } : {})}
          >
            {group.name}
          </div>
        </div>
      )}
      <div className={clsx('gap-[12px] flex flex-col relative', !isOpen && 'hidden')}>
        {group.values.map((integration) => (
          <MenuComponent key={integration.id} integration={integration} collapsed={collapsed} onChange={onChange} />
        ))}
      </div>
    </div>
  );
};

const MenuComponent: FC<{ integration: Channel; collapsed: boolean; onChange: () => void }> = ({
  integration,
  collapsed,
  onChange,
}) => {
  const [, drag, dragPreview] = useDrag(() => ({ type: 'menu', item: { id: integration.id } }), [integration.id]);
  return (
    <div
      ref={(node) => void dragPreview(node)}
      data-testid="channel"
      data-disabled={integration.disabled ? 'true' : 'false'}
      {...(collapsed ? { 'data-tooltip-id': 'tooltip', 'data-tooltip-content': integration.name } : {})}
      className={clsx(
        'flex gap-[12px] items-center bg-newBgColorInner hover:bg-boxHover group/profile transition-all rounded-e-[8px]',
        integration.refreshNeeded && 'cursor-pointer'
      )}
    >
      <div className={clsx('relative gap-[6px] flex justify-center items-center', integration.disabled && 'opacity-50')}>
        <div className="h-full w-[4px] -ms-[12px] rounded-s-[3px] opacity-0 group-hover/profile:opacity-100 transition-opacity">
          <SVGLine />
        </div>
        {(integration.inBetweenSteps || integration.refreshNeeded) && (
          <div className="absolute start-0 top-0 w-[39px] h-[46px] cursor-pointer">
            <div className="bg-red-500 w-[15px] h-[15px] rounded-full start-[5px] top-[5px] absolute z-[200] text-[10px] flex justify-center items-center">
              !
            </div>
            <div className="bg-primary/60 w-[39px] h-[46px] start-0 top-0 absolute rounded-full z-[199]" />
          </div>
        )}
        <img
          src={integration.picture || '/no-picture.jpg'}
          onError={(e) => {
            e.currentTarget.src = '/no-picture.jpg';
          }}
          className="rounded-[8px] min-w-[36px] min-h-[36px]"
          alt={integration.provider}
          width={36}
          height={36}
        />
        <img
          src={`/icons/platforms/${integration.provider}.png`}
          className="rounded-[8px] absolute z-10 bottom-[5px] -end-[5px] border border-fifth"
          alt={integration.provider}
          width={18.41}
          height={18.41}
        />
      </div>
      <div
        ref={(node) => void drag(node)}
        role="Handle"
        className={clsx(
          'group-[.sidebar]:hidden flex-1 whitespace-nowrap text-ellipsis overflow-hidden cursor-move',
          integration.disabled && 'opacity-50'
        )}
      >
        {integration.name}
      </div>
      <Menu integration={integration} onChange={onChange} />
    </div>
  );
};

export function groupChannels(channels: Channel[]): Group[] {
  const sorted = [...channels].sort(
    (a, b) => Number(a.disabled) - Number(b.disabled) || a.provider.localeCompare(b.provider) || a.name.localeCompare(b.name)
  );
  const groups = new Map<string, Group>();
  for (const channel of sorted) {
    const id = channel.customer?.id ?? '';
    const group = groups.get(id) ?? { id, name: channel.customer?.name ?? '', values: [] };
    group.values.push(channel);
    groups.set(id, group);
  }
  return [...groups.values()].sort((a, b) => a.name.localeCompare(b.name));
}

export function ChannelsSidebar() {
  const t = useT();
  const queryClient = useQueryClient();
  const { data: channels = [], isLoading } = useQuery(channelsQuery);
  const [collapseMenu, setCollapseMenu] = useState(() => readFlag('postik_collapse_menu', '0'));
  const refresh = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: channelsQuery.queryKey });
    void queryClient.invalidateQueries({ queryKey: customersQuery.queryKey });
    void queryClient.invalidateQueries({ queryKey: ['posts'] });
  }, [queryClient]);

  const changeItemGroup = useCallback(
    async (id: string, group: string) => {
      queryClient.setQueryData(channelsQuery.queryKey, (current) =>
        current?.map((c) => (c.id === id ? { ...c, customer: group ? { id: group, name: c.customer?.name ?? '' } : undefined } : c))
      );
      await api.PUT('/channels/{id}/customer', {
        params: { path: { id } },
        body: { customerId: group || null },
      });
      refresh();
    },
    [queryClient, refresh]
  );

  const menuIntegrations = useMemo(() => groupChannels(channels), [channels]);
  const toggleCollapse = () => {
    const next = collapseMenu === '1' ? '0' : '1';
    writeFlag('postik_collapse_menu', next);
    setCollapseMenu(next);
  };

  return (
    <div className={clsx('flex relative flex-col', collapseMenu === '1' ? 'group sidebar w-[100px]' : 'w-[260px]')}>
      <div className="bg-newBgColorInner p-[20px] flex flex-col gap-[15px] transition-all absolute start-0 top-0 w-full h-full overflow-x-hidden overflow-y-auto scrollbar scrollbar-thumb-fifth scrollbar-track-newBgColor">
        <div className="flex items-center">
          <h2 className="group-[.sidebar]:hidden flex-1 text-[20px] font-[500]">{t('channels')}</h2>
          <div
            onClick={toggleCollapse}
            className="group-[.sidebar]:rotate-[180deg] group-[.sidebar]:mx-auto text-btnText bg-btnSimple rounded-[6px] w-[24px] h-[24px] flex items-center justify-center cursor-pointer select-none"
          >
            <svg xmlns="http://www.w3.org/2000/svg" width="7" height="13" viewBox="0 0 7 13" fill="none">
              <path d="M6 11.5L1 6.5L6 1.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
        </div>
        <div className="flex flex-col gap-[8px] group-[.sidebar]:mx-auto group-[.sidebar]:w-[44px]">
          <AddProviderButton />
          <div className="flex gap-[8px] group-[.sidebar]:flex-col">{channels.length > 0 && <NewPost />}</div>
        </div>
        <div className="gap-[32px] flex flex-col select-none flex-1">
          {!isLoading && channels.length === 0 && collapseMenu === '0' && (
            <div className="flex-1 max-h-[500px] justify-center items-center flex">
              <div className="flex flex-col gap-[12px] text-center">
                <img
                  src={storedMode() === 'dark' ? '/no-channels.svg' : '/no-channels-colors.svg'}
                  alt="No channels"
                  className="mx-auto min-w-[100%]"
                />
                <div className="font-[600] text-[20px]">{t('no_channels', 'No channels yet')}</div>
                <div className="text-[14px]">{t('connect_your_accounts')}</div>
              </div>
            </div>
          )}
          {menuIntegrations.map((menu) => (
            <MenuGroupComponent
              key={menu.id || 'none'}
              collapsed={collapseMenu === '1'}
              changeItemGroup={changeItemGroup}
              group={menu}
              onChange={refresh}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
