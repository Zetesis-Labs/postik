// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-launch/add.edit.modal.tsx (AGPL-3.0).
import { FC, useEffect } from 'react';
import { dayjs, newDayjs } from '@/lib/dates';
import { useShallow } from '@/lib/store';
import { Integrations } from '@/components/launches/calendar-context';
import { MediaItem, useLaunchStore } from '@/components/new-launch/store';
import { useExistingData } from '@/components/new-launch/existing-data';
import { ManageModal } from '@/components/new-launch/manage-modal';

export interface AddEditModalProps {
  date: dayjs.Dayjs;
  integrations: Integrations[];
  allIntegrations?: Integrations[];
  selectedChannels?: string[];
  mutate: () => void;
  onlyValues?: Array<{ content: string; media?: MediaItem[] }>;
}

export const makeId = () => crypto.randomUUID().slice(0, 10);

const asParagraphs = (content: string) =>
  content.indexOf('<p>') > -1
    ? content
    : content
        .split('\n')
        .map((line) => `<p>${line}</p>`)
        .join('');

export const AddEditModal: FC<AddEditModalProps> = (props) => {
  const { setAllIntegrations, setDate } = useLaunchStore(
    useShallow((state) => ({ setAllIntegrations: state.setAllIntegrations, setDate: state.setDate }))
  );
  const integrations = useLaunchStore((state) => state.integrations);
  useEffect(() => {
    setDate(props.date || newDayjs());
    setAllIntegrations(props.allIntegrations || props.integrations);
  }, []);

  if (!integrations.length) {
    return null;
  }
  return <AddEditModalInner {...props} />;
};

const AddEditModalInner: FC<AddEditModalProps> = (props) => {
  const existingData = useExistingData();
  const { addOrRemoveSelectedIntegration, selectedIntegrations, integrations } = useLaunchStore(
    useShallow((state) => ({
      integrations: state.integrations,
      selectedIntegrations: state.selectedIntegrations,
      addOrRemoveSelectedIntegration: state.addOrRemoveSelectedIntegration,
    }))
  );

  useEffect(() => {
    if (existingData.integration) {
      const integration = integrations.find((i) => i.id === existingData.integration);
      if (integration) {
        addOrRemoveSelectedIntegration(integration, existingData.post?.settings ?? {});
      }
    }
    for (const channel of props.selectedChannels ?? []) {
      const integration = integrations.find((i) => i.id === channel);
      if (integration) {
        addOrRemoveSelectedIntegration(integration, {});
      }
    }
  }, []);

  if (existingData.integration && selectedIntegrations.length === 0) {
    return null;
  }
  return <AddEditModalInnerInner {...props} />;
};

const AddEditModalInnerInner: FC<AddEditModalProps> = (props) => {
  const existingData = useExistingData();
  const { reset, addGlobalValue, addInternalValue, global, setCurrent, internal, setTags, setEditor } = useLaunchStore(
    useShallow((state) => ({
      reset: state.reset,
      addGlobalValue: state.addGlobalValue,
      addInternalValue: state.addInternalValue,
      setCurrent: state.setCurrent,
      global: state.global,
      internal: state.internal,
      setTags: state.setTags,
      setEditor: state.setEditor,
    }))
  );

  useEffect(() => {
    const post = existingData.post;
    if (existingData.integration && post) {
      setTags(post.tags.map((tag) => ({ label: tag.name, value: tag.id })));
      addInternalValue(
        0,
        existingData.integration,
        post.values.map((value) => ({
          id: makeId(),
          delay: value.delayMinutes,
          content: asParagraphs(value.content),
          media: value.media.map((m) => ({ id: m.id, path: m.url, kind: m.kind, alt: m.alt ?? undefined })),
        }))
      );
      setCurrent(existingData.integration);
    } else {
      setEditor('normal');
    }

    addGlobalValue(
      0,
      props.onlyValues?.length
        ? props.onlyValues.map((p) => ({ content: asParagraphs(p.content), id: makeId(), delay: 0, media: p.media || [] }))
        : [{ content: '', id: makeId(), delay: 0, media: [] }]
    );

    return () => {
      reset();
    };
  }, []);

  if (!global.length && !internal.length) {
    return null;
  }
  return <ManageModal {...props} />;
};
