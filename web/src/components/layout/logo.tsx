export const Logo = () => {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="60"
      height="60"
      viewBox="0 0 60 60"
      fill="none"
      role="img"
      aria-label="postik"
      className="mt-[8px] min-w-[60px] min-h-[60px]"
    >
      <rect x="6" y="6" width="48" height="48" rx="14" fill="#612BD3" />
      <path
        d="M23 44V20.5C23 18.567 24.567 17 26.5 17H32C36.9706 17 41 21.0294 41 26C41 30.9706 36.9706 35 32 35H28"
        stroke="white"
        strokeWidth="4.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <circle cx="32" cy="26" r="2.75" fill="white" />
    </svg>
  );
};

export const LogoText = () => {
  return (
    <div className="flex items-center gap-[10px]">
      <svg xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="6 6 48 48" fill="none" aria-hidden="true">
        <rect x="6" y="6" width="48" height="48" rx="14" fill="#612BD3" />
        <path
          d="M23 44V20.5C23 18.567 24.567 17 26.5 17H32C36.9706 17 41 21.0294 41 26C41 30.9706 36.9706 35 32 35H28"
          stroke="white"
          strokeWidth="4.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        <circle cx="32" cy="26" r="2.75" fill="white" />
      </svg>
      <span className="text-[28px] font-[600] -tracking-[0.5px]">postik</span>
    </div>
  );
};
