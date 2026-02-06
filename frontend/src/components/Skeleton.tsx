export function PaperCardSkeleton() {
  return (
    <div className="bg-white dark:bg-surface-900 rounded-xl border border-surface-200 dark:border-surface-800 p-5">
      <div className="flex items-center gap-2 mb-2">
        <div className="skeleton h-5 w-16 rounded-md" />
        <div className="skeleton h-4 w-24 rounded" />
      </div>
      <div className="skeleton h-5 w-full rounded mb-1" />
      <div className="skeleton h-5 w-3/4 rounded mb-2" />
      <div className="skeleton h-4 w-48 rounded mb-3" />
      <div className="skeleton h-4 w-full rounded mb-1" />
      <div className="skeleton h-4 w-full rounded mb-1" />
      <div className="skeleton h-4 w-2/3 rounded" />
    </div>
  );
}

export function PaperDetailSkeleton() {
  return (
    <div className="space-y-4">
      <div className="skeleton h-8 w-3/4 rounded" />
      <div className="flex gap-2">
        <div className="skeleton h-6 w-16 rounded-md" />
        <div className="skeleton h-6 w-32 rounded" />
      </div>
      <div className="skeleton h-4 w-64 rounded" />
      <div className="space-y-2 mt-6">
        <div className="skeleton h-4 w-full rounded" />
        <div className="skeleton h-4 w-full rounded" />
        <div className="skeleton h-4 w-full rounded" />
        <div className="skeleton h-4 w-2/3 rounded" />
      </div>
    </div>
  );
}

export function ListSkeleton({ count = 5 }: { count?: number }) {
  return (
    <div className="space-y-4">
      {Array.from({ length: count }).map((_, i) => (
        <PaperCardSkeleton key={i} />
      ))}
    </div>
  );
}
