// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-launch/select.current.tsx (AGPL-3.0).
import { FC, RefObject, useCallback, useEffect, useRef, useState } from 'react';
import clsx from 'clsx';
import { useT } from '@/lib/i18n';
import { useShallow } from '@/lib/store';
import { GlobalIcon } from '@/components/ui/icons';
import { useDecisionModal } from '@/components/ui/modals';
import { Integrations } from '@/components/launches/calendar-context';
import { useLaunchStore } from '@/components/new-launch/store';

export function useHasScroll(ref: RefObject<HTMLElement | null>): boolean {
  const [hasHorizontalScroll, setHasHorizontalScroll] = useState(false);
  useEffect(() => {
    if (!ref.current) return;
    const checkScroll = () => {
      const el = ref.current;
      if (el) {
        setHasHorizontalScroll(el.scrollWidth > el.clientWidth);
      }
    };
    checkScroll();
    const resizeObserver = new ResizeObserver(checkScroll);
    resizeObserver.observe(ref.current);
    const mutationObserver = new MutationObserver(checkScroll);
    mutationObserver.observe(ref.current, { childList: true, subtree: true, characterData: true });
    return () => {
      resizeObserver.disconnect();
      mutationObserver.disconnect();
    };
  }, [ref]);
  return hasHorizontalScroll;
}

export const SelectCurrent: FC = () => {
  const decision = useDecisionModal();
  const { selectedIntegrations, current, setCurrent, locked, setHide, addOrRemoveSelectedIntegration } = useLaunchStore(
    useShallow((state) => ({
      selectedIntegrations: state.selectedIntegrations,
      addOrRemoveSelectedIntegration: state.addOrRemoveSelectedIntegration,
      current: state.current,
      setCurrent: state.setCurrent,
      locked: state.locked,
      setHide: state.setHide,
    }))
  );
  const contentRef = useRef<HTMLDivElement>(null);
  const hasScroll = useHasScroll(contentRef);

  const removeSocial = useCallback(
    (sIntegration: Integrations) => async (e: { stopPropagation: () => void; preventDefault: () => void }) => {
      e.stopPropagation();
      e.preventDefault();
      const confirmed = await decision.open({
        title: 'Remove Social Account',
        description: 'Are you sure you want to remove this social from scheduling?',
      });
      if (!confirmed) {
        return;
      }
      addOrRemoveSelectedIntegration(sIntegration, {});
    },
    [addOrRemoveSelectedIntegration, decision]
  );

  return (
    <>
      <div className="select-none left-0 absolute w-full z-[100] px-[20px]">
        <div
          ref={contentRef}
          className={clsx(
            'flex gap-[6px] w-full overflow-x-auto scrollbar scrollbar-thumb-tableBorder scrollbar-track-secondary',
            locked && 'opacity-50 pointer-events-none'
          )}
        >
          <div
            onClick={() => {
              setHide(true);
              setCurrent('global');
            }}
            className={clsx(
              'cursor-pointer flex gap-[8px] rounded-[8px] w-[40px] h-[40px] justify-center items-center bg-newBgLineColor',
              current !== 'global' ? 'text-[#A3A3A3]' : 'border border-[#FC69FF] text-[#FC69FF]'
            )}
          >
            <div>
              <GlobalIcon />
            </div>
          </div>
          {selectedIntegrations.map(({ integration }) => (
            <div
              onClick={() => {
                setHide(true);
                setCurrent(integration.id);
              }}
              key={integration.id}
              data-testid="select-current-channel"
              className={clsx(
                'border cursor-pointer relative flex gap-[8px] w-[40px] h-[40px] rounded-[8px] items-center bg-newBgLineColor justify-center',
                current === integration.id ? 'border-[#FC69FF] text-[#FC69FF]' : 'border-transparent'
              )}
            >
              <div
                onClick={removeSocial(integration)}
                className="absolute justify-center items-center flex w-[8px] h-[8px] -top-[1px] -start-[3px] bg-red-500 rounded-full text-white text-[8px]"
              >
                X
              </div>
              <IsGlobal id={integration.id} />
              <div
                data-tooltip-id="tooltip"
                data-tooltip-content={integration.name}
                className="relative w-full h-full rounded-full flex justify-center items-center filter transition-all duration-500"
              >
                <img
                  src={integration.picture || '/no-picture.jpg'}
                  className="rounded-full min-w-[26px]"
                  alt={integration.identifier}
                  width={26}
                  height={26}
                  onError={(e) => {
                    e.currentTarget.src = '/no-picture.jpg';
                  }}
                />
                <img
                  src={`/icons/platforms/${integration.identifier}.png`}
                  className="min-w-[12px] min-h-[12px] rounded-[3px] absolute z-10 bottom-[6px] end-[6px]"
                  alt={integration.identifier}
                  width={12}
                  height={12}
                />
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className={clsx(hasScroll ? 'h-[55px]' : 'h-[40px]')} />
    </>
  );
};

export const IsGlobal: FC<{ id: string }> = ({ id }) => {
  const t = useT();
  const isInternal = useLaunchStore((state) => !!state.internal.find((p) => p.integration.id === id));
  if (!isInternal) {
    return null;
  }
  return (
    <div
      data-tooltip-id="tooltip"
      data-tooltip-content={t('no_longer_global_mode', 'No longer in global mode')}
      className="w-[8px] h-[8px] bg-[#FC69FF] -top-[1px] -end-[3px] absolute rounded-full"
    />
  );
};
