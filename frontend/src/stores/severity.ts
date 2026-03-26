import { derived, get, writable } from 'svelte/store';

export interface SeverityLevel {
  name: string;
  color: string;
  aliases: string[];
  rank: number;
}

export interface SeverityConfig {
  default: string;
  levels: SeverityLevel[];
}

const defaultSeverityConfig: SeverityConfig = {
  default: 'unknown',
  levels: [
    { name: 'critical', color: '#ef4444', aliases: ['critical'], rank: 0 },
    { name: 'warning', color: '#f59e0b', aliases: ['warning'], rank: 1 },
    { name: 'info', color: '#3b82f6', aliases: ['info'], rank: 2 },
    { name: 'unknown', color: '#6b7280', aliases: ['unknown'], rank: 3 },
  ],
};

export const severityConfig = writable<SeverityConfig>(defaultSeverityConfig);
export const severityLevels = derived(severityConfig, $config => $config.levels ?? []);

export function normalizeSeverityName(value: string): string {
  return value?.trim().toLowerCase() ?? '';
}

export function setSeverityConfig(config?: Partial<SeverityConfig>): void {
  const levels = (config?.levels ?? [])
    .filter(level => normalizeSeverityName(level?.name ?? '') !== '')
    .map((level, index) => {
      const name = normalizeSeverityName(level.name);
      const aliasSet = new Set<string>([name]);
      for (const alias of level.aliases ?? []) {
        const normalized = normalizeSeverityName(alias);
        if (normalized) aliasSet.add(normalized);
      }
      return {
        name,
        color: level.color?.trim() || defaultColorForSeverity(name),
        aliases: [...aliasSet],
        rank: Number.isFinite(level.rank) ? level.rank : index,
      };
    });

  severityConfig.set({
    default: normalizeSeverityName(config?.default ?? '') || (levels.find(level => level.name === 'unknown')?.name ?? levels[levels.length - 1]?.name ?? defaultSeverityConfig.default),
    levels: levels.length > 0 ? levels : defaultSeverityConfig.levels,
  });
}

export function severityOrder(severity: string): number {
  const config = get(severityConfig);
  const canonical = canonicalSeverity(severity, config);
  const match = config.levels.find(level => level.name === canonical);
  return match?.rank ?? config.levels.length;
}

export function severityColor(severity: string): string {
  const config = get(severityConfig);
  const canonical = canonicalSeverity(severity, config);
  return config.levels.find(level => level.name === canonical)?.color ?? defaultColorForSeverity(config.default);
}

export function canonicalSeverity(severity: string, config: SeverityConfig = get(severityConfig)): string {
  const target = normalizeSeverityName(severity);
  for (const level of config.levels) {
    if (level.name === target) return level.name;
    if ((level.aliases ?? []).some(alias => alias === target)) return level.name;
  }
  return config.default;
}

export function emptySeverityCounts(): Record<string, number> {
  const config = get(severityConfig);
  const counts: Record<string, number> = {};
  for (const level of config.levels) {
    counts[level.name] = 0;
  }
  return counts;
}

export function severityLabel(name: string): string {
  const value = normalizeSeverityName(name);
  if (!value) return 'Unknown';
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function defaultColorForSeverity(name: string): string {
  switch (normalizeSeverityName(name)) {
    case 'critical': return '#ef4444';
    case 'warning': return '#f59e0b';
    case 'info': return '#3b82f6';
    default: return '#6b7280';
  }
}
