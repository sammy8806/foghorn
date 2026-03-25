export type Severity = 'critical' | 'warning' | 'info' | 'unknown';

export function severityOrder(severity: string): number {
  switch (severity?.toLowerCase()) {
    case 'critical': return 0;
    case 'warning': return 1;
    case 'info': return 2;
    default: return 3;
  }
}

export function severityClass(severity: string): string {
  switch (severity?.toLowerCase()) {
    case 'critical': return 'severity-critical';
    case 'warning': return 'severity-warning';
    case 'info': return 'severity-info';
    default: return 'severity-unknown';
  }
}

export function severityColor(severity: string): string {
  switch (severity?.toLowerCase()) {
    case 'critical': return '#ef4444';
    case 'warning': return '#f59e0b';
    case 'info': return '#3b82f6';
    default: return '#6b7280';
  }
}

export function formatDuration(startTime: string): string {
  const start = new Date(startTime);
  const now = new Date();
  const diffMs = now.getTime() - start.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 0) return `${diffDays}d ${diffHours % 24}h`;
  if (diffHours > 0) return `${diffHours}h ${diffMins % 60}m`;
  if (diffMins > 0) return `${diffMins}m`;
  return 'just now';
}
