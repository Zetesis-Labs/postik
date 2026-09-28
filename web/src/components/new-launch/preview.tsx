// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/general.preview.component.tsx
// y apps/frontend/src/components/new-launch/providers/show.all.providers.tsx (AGPL-3.0).
// Solo la vista previa general: sin menciones ni vistas propias por red.
import { FC } from 'react';
import clsx from 'clsx';
import { useT } from '@/lib/i18n';
import { useShallow } from '@/lib/store';
import { characterLimit, plainText } from '@/lib/post-text';
import { VideoFrame } from '@/components/media/media-box';
import { Integrations } from '@/components/launches/calendar-context';
import { useLaunchStore, Values } from '@/components/new-launch/store';

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

// previewHtml marks what goes past the limit, which the channel would reject.
function previewHtml(content: string, limit: number): string {
  const text = plainText(content.replace(/<\/p>/g, '</p>\n')).replace(/\n$/, '');
  const end = limit > 0 ? limit : text.length;
  const overflow = text.slice(end);
  return (
    escapeHtml(text.slice(0, end)) +
    (overflow
      ? `<mark class="bg-red-500" data-tooltip-id="tooltip" data-tooltip-content="This text will be cropped">${escapeHtml(overflow)}</mark>`
      : '')
  );
}

const GeneralPreviewComponent: FC<{ integration?: Integrations; values: Values[]; isGlobal: boolean; maximumCharacters: number }> = ({
  integration,
  values,
  isGlobal,
  maximumCharacters,
}) => (
  <div className="w-full p-[15px]" data-testid="post-preview">
    <div className="w-full h-full relative flex flex-col">
      {values.map((value, index) => (
        <div key={value.id} className={clsx('flex gap-[8px] relative', index === values.length - 1 ? 'pb-[12px]' : 'pb-[24px]')}>
          <div className="min-w-[40px] h-[40px] min-h-[40px] w-[40px] flex flex-col items-center">
            <div className="relative">
              <img src={isGlobal ? '/no-picture.jpg' : integration?.picture || '/no-picture.jpg'} alt="x" className="rounded-full relative z-[2]" />
              {!isGlobal && integration && (
                <img
                  src={`/icons/platforms/${integration.identifier}.png`}
                  className="min-w-[20px] min-h-[20px] rounded-full absolute z-10 -bottom-[5px] -end-[5px] border border-fifth"
                  alt={integration.identifier}
                  width={20}
                  height={20}
                />
              )}
            </div>
            {index !== values.length - 1 && <div className="flex-1 w-[2px] h-[calc(100%-10px)] bg-customColor25 absolute top-[10px] z-[1]" />}
          </div>
          <div className="flex-1 flex flex-col gap-[4px]">
            <div className="flex">
              <div className="h-[22px] text-[15px] font-[700]">{isGlobal ? 'Global Edit' : integration?.name}</div>
              <div className="text-[15px] text-customColor26 mt-[1px] ms-[2px]">
                <svg viewBox="0 0 22 22" aria-label="Verified account" role="img" className="max-w-[20px] max-h-[20px] fill-current h-[1.25em]">
                  <g>
                    <path d="M20.396 11c-.018-.646-.215-1.275-.57-1.816-.354-.54-.852-.972-1.438-1.246.223-.607.27-1.264.14-1.897-.131-.634-.437-1.218-.882-1.687-.47-.445-1.053-.75-1.687-.882-.633-.13-1.29-.083-1.897.14-.273-.587-.704-1.086-1.245-1.44S11.647 1.62 11 1.604c-.646.017-1.273.213-1.813.568s-.969.854-1.24 1.44c-.608-.223-1.267-.272-1.902-.14-.635.13-1.22.436-1.69.882-.445.47-.749 1.055-.878 1.688-.13.633-.08 1.29.144 1.896-.587.274-1.087.705-1.443 1.245-.356.54-.555 1.17-.574 1.817.02.647.218 1.276.574 1.817.356.54.856.972 1.443 1.245-.224.606-.274 1.263-.144 1.896.13.634.433 1.218.877 1.688.47.443 1.054.747 1.687.878.633.132 1.29.084 1.897-.136.274.586.705 1.084 1.246 1.439.54.354 1.17.551 1.816.569.647-.016 1.276-.213 1.817-.567s.972-.854 1.245-1.44c.604.239 1.266.296 1.903.164.636-.132 1.22-.447 1.68-.907.46-.46.776-1.044.908-1.681s.075-1.299-.165-1.903c.586-.274 1.084-.705 1.439-1.246.354-.54.551-1.17.569-1.816zM9.662 14.85l-3.429-3.428 1.293-1.302 2.072 2.072 4.4-4.794 1.347 1.246z" />
                  </g>
                </svg>
              </div>
              <div className="text-[15px] font-[400] text-customColor27 ms-[4px]">{isGlobal ? '' : integration?.display}</div>
            </div>
            <div className="text-wrap whitespace-pre preview" dangerouslySetInnerHTML={{ __html: previewHtml(value.content, maximumCharacters) }} />
            {!!value.media?.length && (
              <div className={clsx('w-full rounded-[16px] overflow-hidden mt-[12px]', value.media.length > 3 ? 'grid grid-cols-2 gap-[4px]' : 'flex gap-[4px]')}>
                {value.media.map((media) => (
                  <a key={media.id} className="flex-1" href={media.path} target="_blank" rel="noreferrer">
                    {media.kind === 'video' ? <VideoFrame url={media.path} autoplay={true} /> : <img className="w-full h-full object-cover" src={media.path} alt={media.alt || ''} />}
                  </a>
                ))}
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  </div>
);

export const ShowAllProviders: FC = () => {
  const t = useT();
  const { current, global, internal, selectedIntegrations } = useLaunchStore(
    useShallow((state) => ({
      current: state.current,
      global: state.global,
      internal: state.internal,
      selectedIntegrations: state.selectedIntegrations,
    }))
  );
  const isGlobal = current === 'global';
  const integration = isGlobal ? selectedIntegrations[0]?.integration : selectedIntegrations.find((p) => p.integration.id === current)?.integration;
  const values = isGlobal ? global : (internal.find((p) => p.integration.id === current)?.integrationValue ?? global);

  if (!values[0]?.content?.length && !values[0]?.media?.length) {
    return <div className="w-full flex flex-col flex-1">{t('start_writing_your_post', 'Start writing your post for a preview')}</div>;
  }
  return (
    <div className="w-full flex flex-col flex-1">
      <div className="border border-borderPreview rounded-[12px] shadow-previewShadow">
        <GeneralPreviewComponent
          integration={integration}
          values={values}
          isGlobal={isGlobal}
          maximumCharacters={isGlobal ? 0 : characterLimit(integration?.identifier ?? '')}
        />
      </div>
    </div>
  );
};
