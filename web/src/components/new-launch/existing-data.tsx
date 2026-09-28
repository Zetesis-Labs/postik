// Adaptado de Postiz v2.24.0: apps/frontend/src/components/launches/helpers/use.existing.data.tsx (AGPL-3.0).
import { createContext, FC, ReactNode, useContext } from 'react';
import type { PostDetail } from '@/lib/api';

type ExistingData = {
  integration: string;
  group?: string;
  post?: PostDetail;
};

const ExistingDataContext = createContext<ExistingData>({ integration: '' });

export const ExistingDataContextProvider: FC<{ children: ReactNode; value: ExistingData }> = ({ children, value }) => (
  <ExistingDataContext.Provider value={value}>{children}</ExistingDataContext.Provider>
);

export const useExistingData = () => useContext(ExistingDataContext);
