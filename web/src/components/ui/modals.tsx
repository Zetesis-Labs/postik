// Adaptado de Postiz v2.24.0: apps/frontend/src/components/layout/new-modal.tsx (AGPL-3.0).
// El estado de los modales es propio: un almacén mínimo con useSyncExternalStore en vez de zustand.
import {
  createContext,
  FC,
  memo,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useSyncExternalStore,
} from 'react';
import clsx from 'clsx';
import i18next from 'i18next';
import { Button } from '@/components/ui/button';

interface OpenModalInterface {
  title?: ReactNode;
  closeOnClickOutside?: boolean;
  removeLayout?: boolean;
  fullScreen?: boolean;
  top?: string | number;
  withCloseButton?: boolean;
  askClose?: boolean;
  onClose?: () => void;
  children: ReactNode | ((close: () => void) => ReactNode);
  size?: string | number;
  maxSize?: string | number;
  height?: string | number;
  id?: string;
}

type OpenModal = { id: string } & OpenModalInterface;

let modals: OpenModal[] = [];
const listeners = new Set<() => void>();

function emit(next: OpenModal[]) {
  modals = next;
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function makeId() {
  return crypto.randomUUID();
}

const modalStore = {
  open(params: OpenModalInterface) {
    const id = params.id || makeId();
    if (!modals.some((m) => m.id === id)) {
      emit([...modals, { id, ...params }]);
    }
  },
  closeById(id: string) {
    emit(modals.filter((m) => m.id !== id));
  },
  closeAll() {
    emit([]);
  },
};

const CurrentModalContext = createContext({ id: '' });

export const useModals = () => {
  const modalContext = useContext(CurrentModalContext);
  return useMemo(
    () => ({
      openModal: modalStore.open,
      closeAll: modalStore.closeAll,
      closeById: modalStore.closeById,
      closeCurrent: () => {
        if (modalContext.id) {
          modalStore.closeById(modalContext.id);
        }
      },
    }),
    [modalContext.id]
  );
};

const CloseIcon = () => (
  <svg viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" width="16" height="16">
    <path
      d="M11.7816 4.03157C12.0062 3.80702 12.0062 3.44295 11.7816 3.2184C11.5571 2.99385 11.193 2.99385 10.9685 3.2184L7.50005 6.68682L4.03164 3.2184C3.80708 2.99385 3.44301 2.99385 3.21846 3.2184C2.99391 3.44295 2.99391 3.80702 3.21846 4.03157L6.68688 7.49999L3.21846 10.9684C2.99391 11.193 2.99391 11.557 3.21846 11.7816C3.44301 12.0061 3.80708 12.0061 4.03164 11.7816L7.50005 8.31316L10.9685 11.7816C11.193 12.0061 11.5571 12.0061 11.7816 11.7816C12.0062 11.557 12.0062 11.193 11.7816 10.9684L8.31322 7.49999L11.7816 4.03157Z"
      fill="currentColor"
      fillRule="evenodd"
      clipRule="evenodd"
    ></path>
  </svg>
);

const ModalComponent: FC<{
  isLast: boolean;
  modal: OpenModal;
  zIndex: number;
}> = memo(({ isLast, modal, zIndex }) => {
  const decision = useDecisionModal();
  const closeModalFunction = useCallback(async () => {
    if (modal.askClose) {
      const confirmed = await decision.open();
      if (!confirmed) {
        return;
      }
    }
    modal.onClose?.();
    modalStore.closeById(modal.id);
  }, [modal, decision]);

  useEffect(() => {
    if (!isLast) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        closeModalFunction();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [isLast, closeModalFunction]);

  const content = typeof modal.children === 'function' ? modal.children(closeModalFunction) : modal.children;

  if (modal.removeLayout) {
    return (
      <CurrentModalContext.Provider value={{ id: modal.id }}>
        <div
          style={{ zIndex }}
          className={clsx(
            !modal.fullScreen ? 'pb-[50px] min-w-full min-h-full' : 'w-full h-full',
            'fixed flex left-0 top-0 bg-popup transition-all animate-fadeIn overflow-y-auto text-newTextColor',
            !isLast && '!overflow-hidden'
          )}
        >
          <div className={clsx(modal.fullScreen && 'flex', 'relative flex-1')}>
            <div className={clsx(modal.fullScreen ? 'flex flex-1' : 'absolute top-0 left-0 min-w-full min-h-full')}>
              <div
                className={clsx(modal.fullScreen ? 'w-full h-full flex-1' : 'mx-auto py-[48px]')}
                {...(modal.size && { style: { width: modal.size } })}
              >
                {content}
              </div>
            </div>
          </div>
        </div>
      </CurrentModalContext.Provider>
    );
  }

  return (
    <CurrentModalContext.Provider value={{ id: modal.id }}>
      <div
        onClick={closeModalFunction}
        style={{ zIndex }}
        className={clsx(
          'fixed flex left-0 top-0 min-w-full min-h-full bg-popup transition-all animate-fadeIn overflow-y-auto text-newTextColor',
          !modal.fullScreen && 'pb-[50px]'
        )}
      >
        <div className="relative flex-1">
          <div
            style={modal.top ? { paddingTop: modal.top, paddingBottom: modal.top } : {}}
            className={clsx(
              'absolute min-w-full',
              !modal.fullScreen ? (modal.top ? '' : 'min-h-full pt-[100px] pb-[100px]') : 'h-screen',
              modal.size && modal.height ? 'flex justify-center items-center' : 'top-0 left-0'
            )}
          >
            <div
              role="dialog"
              className={clsx(
                !modal.removeLayout && 'gap-[40px] p-[32px]',
                'bg-newBgColorInner mx-auto flex flex-col w-fit rounded-[24px] relative',
                modal.size ? '' : 'min-w-[600px]',
                modal.fullScreen && 'h-full'
              )}
              {...((!!modal.size || !!modal.height || !!modal.maxSize) && {
                style: {
                  ...(modal.size ? { width: modal.size } : {}),
                  ...(modal.height ? { height: modal.height } : {}),
                  ...(modal.maxSize ? { maxWidth: modal.maxSize } : {}),
                },
              })}
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center">
                <div className="text-[24px] font-[600] flex-1">{modal.title}</div>
                {typeof modal.withCloseButton === 'undefined' || modal.withCloseButton ? (
                  <div className="cursor-pointer">
                    <button
                      className="outline-none absolute end-[20px] top-[20px] hover:bg-tableBorder cursor-pointer"
                      type="button"
                      aria-label={i18next.t('close', 'Close')}
                      onClick={closeModalFunction}
                    >
                      <CloseIcon />
                    </button>
                  </div>
                ) : null}
              </div>
              <div className={clsx('whitespace-pre-line', !!modal.height && !!modal.size && 'flex flex-1 flex-col')}>
                {content}
              </div>
            </div>
          </div>
        </div>
      </div>
    </CurrentModalContext.Provider>
  );
});

const ModalManagerInner: FC = () => {
  const current = useSyncExternalStore(subscribe, () => modals);

  useEffect(() => {
    document.body.classList.toggle('overflow-hidden', current.length > 0);
    document.querySelectorAll('.blurMe').forEach((node) => {
      node.classList.toggle('blur-xs', current.length > 0);
      node.classList.toggle('pointer-events-none', current.length > 0);
    });
  }, [current]);

  if (current.length === 0) {
    return null;
  }
  return (
    <>
      <style>{`body, html { overflow: hidden !important; }`}</style>
      {current.map((modal, index) => (
        <ModalComponent isLast={current.length - 1 === index} key={modal.id} modal={modal} zIndex={200 + index} />
      ))}
    </>
  );
};

export const ModalManager: FC<{ children: ReactNode }> = ({ children }) => {
  return (
    <div>
      <ModalManagerInner />
      <div className="transition-all w-full">{children}</div>
    </div>
  );
};

const DecisionModal: FC<{
  description: ReactNode;
  approveLabel: string;
  cancelLabel: string;
  onlyApprove: boolean;
  resolution: (value: boolean) => void;
}> = ({ description, cancelLabel, approveLabel, resolution, onlyApprove }) => {
  const { closeCurrent } = useModals();
  return (
    <div className="flex flex-col">
      <div className="max-w-[600px]">{description}</div>
      <div className="flex gap-[12px] mt-[16px]">
        <Button
          onClick={() => {
            resolution(true);
            closeCurrent();
          }}
        >
          {approveLabel}
        </Button>
        {!onlyApprove && (
          <Button
            onClick={() => {
              resolution(false);
              closeCurrent();
            }}
          >
            {cancelLabel}
          </Button>
        )}
      </div>
    </div>
  );
};

export const useDecisionModal = () => {
  const modalsApi = useModals();
  const open = useCallback(
    ({
      title = i18next.t('are_you_sure', 'Are you sure?') as ReactNode,
      description = i18next.t('are_you_sure_you_want_to_close_this_modal', 'Are you sure you want to close this modal?') as ReactNode,
      onlyApprove = false,
      approveLabel = i18next.t('yes', 'Yes'),
      cancelLabel = i18next.t('no', 'No'),
    } = {}) =>
      new Promise<boolean>((resolve) => {
        modalsApi.openModal({
          title,
          askClose: false,
          onClose: () => resolve(false),
          children: (
            <DecisionModal
              onlyApprove={onlyApprove}
              resolution={resolve}
              description={description}
              approveLabel={approveLabel}
              cancelLabel={cancelLabel}
            />
          ),
        });
      }),
    [modalsApi]
  );
  return { open };
};

// Adaptado de Postiz v2.24.0: libraries/react-shared-libraries/src/helpers/delete.dialog.tsx (AGPL-3.0).
export const deleteDialog = (message: string, confirmButton?: string, title?: string, cancelButton?: string) =>
  new Promise<boolean>((resolve) => {
    modalStore.open({
      title: title || i18next.t('are_you_sure', 'Are you sure?'),
      askClose: false,
      onClose: () => resolve(false),
      children: (
        <DecisionModal
          onlyApprove={false}
          resolution={resolve}
          description={message}
          approveLabel={confirmButton || i18next.t('yes_delete_it', 'Yes, delete it!')}
          cancelLabel={cancelButton || i18next.t('no_cancel', 'No, cancel!')}
        />
      ),
    });
  });
