// Adaptado de Postiz v2.24.0: apps/frontend/src/components/new-layout/menu-item.tsx (AGPL-3.0).
import { FC, ReactNode } from 'react';
import clsx from 'clsx';
import { Link, useLocation } from '@tanstack/react-router';

const itemClassName = (isActive: boolean) =>
  clsx(
    'group w-full minCustom:h-[54px] custom:h-[44px] py-[8px] px-[6px] minCustom:gap-[4px] custom:gap-[2px] flex flex-col font-[600] items-center justify-center rounded-[12px] hover:text-textItemFocused hover:bg-boxFocused transition-colors',
    isActive ? 'text-textItemFocused bg-boxFocused' : 'text-textItemBlur'
  );

const Inner: FC<{ label: string; icon: ReactNode }> = ({ label, icon }) => (
  <>
    <div className="custom:scale-90 transition-transform">{icon}</div>
    <div className="custom:text-[9px] minCustom:text-[10px] leading-[1.1] text-center">{label}</div>
  </>
);

export const MenuItem: FC<{ label: string; icon: ReactNode; path?: string; onClick?: () => void }> = ({
  label,
  icon,
  path,
  onClick,
}) => {
  const location = useLocation();
  const isActive = !!path && location.pathname.indexOf(path) === 0;

  if (onClick || !path) {
    return (
      <button type="button" onClick={onClick} title={label} className={itemClassName(isActive)}>
        <Inner label={label} icon={icon} />
      </button>
    );
  }
  return (
    <Link to={path} title={label} className={itemClassName(isActive)}>
      <Inner label={label} icon={icon} />
    </Link>
  );
};
