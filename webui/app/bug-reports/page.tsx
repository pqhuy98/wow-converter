import type { Metadata } from 'next';

import { BugReports } from '@/components/bug-reports/bug-reports';

export const metadata: Metadata = {
  title: 'Bug Reports',
  robots: { index: false, follow: false },
};

export default function BugReportsPage() { return <BugReports />; }
