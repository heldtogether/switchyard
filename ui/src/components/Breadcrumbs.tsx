import React from "react";
import { Link } from "react-router-dom";

export interface BreadcrumbItem {
  label: string;
  to?: string;
}

interface BreadcrumbsProps {
  items: BreadcrumbItem[];
}

export function Breadcrumbs({ items }: BreadcrumbsProps) {
  if (items.length === 0) return null;

  return (
    <nav aria-label="Breadcrumb">
      <ol className="flex flex-wrap items-center gap-2 text-xs font-medium text-ink-500">
        {items.map((item, index) => {
          const isCurrent = index === items.length - 1;
          return (
            <React.Fragment key={`${item.label}-${index}`}>
              {index > 0 && <li aria-hidden="true" className="text-ink-300">/</li>}
              <li>
                {isCurrent || !item.to ? (
                  <span aria-current="page" className="text-ink-700">
                    {item.label}
                  </span>
                ) : (
                  <Link to={item.to} className="hover:text-ink-900 hover:underline">
                    {item.label}
                  </Link>
                )}
              </li>
            </React.Fragment>
          );
        })}
      </ol>
    </nav>
  );
}
