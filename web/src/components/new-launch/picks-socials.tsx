// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-launch/picks.socials.component.tsx (AGPL-3.0).
import { FC } from 'react';
import clsx from 'clsx';
import { useShallow } from '@/lib/store';
import { useLaunchStore } from '@/components/new-launch/store';
import { useExistingData } from '@/components/new-launch/existing-data';

export const PicksSocialsComponent: FC<{ toolTip?: boolean }> = ({ toolTip }) => {
  const existing = useExistingData();
  const { locked, addOrRemoveSelectedIntegration, integrations, selectedIntegrations } = useLaunchStore(
    useShallow((state) => ({
      integrations: state.integrations,
      selectedIntegrations: state.selectedIntegrations,
      addOrRemoveSelectedIntegration: state.addOrRemoveSelectedIntegration,
      locked: state.locked,
    }))
  );

  return (
    <div className={clsx('flex', locked && 'opacity-50 pointer-events-none')}>
      <div className="flex flex-1">
        <div className="innerComponent flex-1 flex">
          <div className="flex flex-wrap gap-[12px] flex-1">
            {integrations
              .filter((f) => (existing.integration ? f.id === existing.integration : !f.inBetweenSteps && !f.disabled))
              .map((integration) => {
                const selected = selectedIntegrations.some((p) => p.integration.id === integration.id);
                return (
                  <div
                    key={integration.id}
                    className="flex gap-[8px] items-center"
                    {...(toolTip && { 'data-tooltip-id': 'tooltip', 'data-tooltip-content': integration.name })}
                  >
                    <div
                      role="checkbox"
                      aria-checked={selected}
                      aria-label={integration.name}
                      data-testid="pick-channel"
                      onClick={() => {
                        if (existing.integration) {
                          return;
                        }
                        addOrRemoveSelectedIntegration(integration, {});
                      }}
                      className={clsx(
                        'cursor-pointer border-[2px] relative rounded-full flex justify-center items-center bg-fifth filter transition-all duration-500',
                        selected ? 'border-[#622FF6]' : 'grayscale border-transparent'
                      )}
                    >
                      <img
                        src={integration.picture || '/no-picture.jpg'}
                        onError={(e) => {
                          e.currentTarget.src = '/no-picture.jpg';
                        }}
                        className={clsx(
                          'rounded-full transition-all min-w-[42px] border-[1.5px] min-h-[42px]',
                          selected ? 'border-[#000]' : 'border-transparent'
                        )}
                        alt={integration.identifier}
                        width={42}
                        height={42}
                      />
                      <img
                        src={`/icons/platforms/${integration.identifier}.png`}
                        className="rounded-[4px] absolute z-10 bottom-0 -end-[5px] min-w-[16px] min-h-[16px]"
                        alt={integration.identifier}
                        width={16}
                        height={16}
                      />
                    </div>
                  </div>
                );
              })}
          </div>
        </div>
      </div>
    </div>
  );
};
