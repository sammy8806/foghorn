/** Shared Wails bridge mock for PR screenshot captures. */

export function sampleAlerts() {
  return [
    {
      id: '1',
      source: 'prod-am',
      sourceType: 'alertmanager',
      name: 'DiskSpaceLow',
      severity: 'critical',
      state: 'firing',
      labels: { alertname: 'DiskSpaceLow', severity: 'critical', cluster: 'prod-us-east', namespace: 'monitoring', pod: 'prometheus-0' },
      annotations: { summary: 'Disk usage above 90%', description: 'Root volume on prometheus-0 is nearly full', runbook: 'https://wiki.example/runbooks/disk' },
      startsAt: new Date(Date.now() - 3600000).toISOString(),
      updatedAt: new Date().toISOString(),
      generatorURL: 'https://grafana.example.com/alerting/list',
      silencedBy: [],
      inhibitedBy: [],
      receivers: ['pager'],
    },
    {
      id: '2',
      source: 'prod-am',
      sourceType: 'alertmanager',
      name: 'HighMemoryUsage',
      severity: 'warning',
      state: 'firing',
      labels: { alertname: 'HighMemoryUsage', severity: 'warning', cluster: 'prod-us-east', namespace: 'payments', pod: 'api-7f2c9' },
      annotations: { summary: 'Memory above 85%', description: 'payments/api pod memory sustained high for 15m' },
      startsAt: new Date(Date.now() - 900000).toISOString(),
      updatedAt: new Date().toISOString(),
      generatorURL: 'https://grafana.example.com/alerting/list',
      silencedBy: ['silence-abc123'],
      silences: [{ id: 'silence-abc123', createdBy: 'oncall@example.com', comment: 'Known rollout', startsAt: new Date().toISOString(), endsAt: new Date(Date.now() + 7200000).toISOString(), matchers: [] }],
      inhibitedBy: [],
      receivers: ['slack'],
    },
    {
      id: '3',
      source: 'grafana',
      sourceType: 'grafana',
      name: 'DatabaseConnectionsHigh',
      severity: 'warning',
      state: 'firing',
      labels: { alertname: 'DatabaseConnectionsHigh', severity: 'warning', cluster: 'staging', namespace: 'data' },
      annotations: { summary: 'Postgres connections elevated' },
      startsAt: new Date(Date.now() - 1800000).toISOString(),
      updatedAt: new Date().toISOString(),
      generatorURL: 'https://grafana.example.com/alerting/grafana/db/view',
      silencedBy: [],
      inhibitedBy: [],
      receivers: ['email'],
    },
    {
      id: '4',
      source: 'prod-am',
      sourceType: 'alertmanager',
      name: 'SyntheticCheckFailed',
      severity: 'info',
      state: 'firing',
      labels: { alertname: 'SyntheticCheckFailed', severity: 'info', cluster: 'prod-us-west' },
      annotations: { summary: 'External probe failing intermittently' },
      startsAt: new Date(Date.now() - 300000).toISOString(),
      updatedAt: new Date().toISOString(),
      generatorURL: '',
      silencedBy: [],
      inhibitedBy: [],
      receivers: [],
      hiddenBy: ['synthetic noise'],
    },
  ];
}

export function healthySources() {
  const now = new Date().toISOString();
  return [
    { source: 'prod-am', ok: true, pending: false, lastPoll: now, consecFails: 0 },
    { source: 'grafana', ok: true, pending: false, lastPoll: now, consecFails: 0 },
  ];
}

export function failingSources() {
  const now = new Date().toISOString();
  return [
    { source: 'prod-am', ok: false, pending: false, lastPoll: now, lastError: 'connection refused: localhost:9093', consecFails: 4 },
    { source: 'grafana', ok: true, pending: false, lastPoll: now, consecFails: 0 },
  ];
}

export function buildBridgeInit(scenario) {
  const alerts = sampleAlerts();
  const uiConfig = {
    theme: scenario.theme || 'dark',
    popup_width: 500,
    popup_height: 600,
    popup_position: 'top_right',
    show_resolved: scenario.showResolved ?? false,
    show_silenced: scenario.showSilenced ?? false,
    default_created_by: 'screenshot-demo',
    idle_image: '',
    scale: { factor: 1, mode: 'fonts', apply_to_popup: true },
    silence_editor: { always_visible_matchers: ['alertname', 'cluster'], collapse_matchers: true },
  };

  const actions = scenario.actions || [];
  const health = scenario.health === 'failing' ? failingSources() : healthySources();

  return `
    window.go = {
      main: {
        App: {
          GetAlerts: async () => ${JSON.stringify(alerts)},
          GetSeverityCounts: async () => ({ critical: 1, warning: 2, info: 1, unknown: 0 }),
          GetSourcesHealth: async () => ${JSON.stringify(health)},
          GetOnCallStatus: async () => [{ source: 'betterstack', scheduleID: 'default', scheduleName: 'Primary', teamName: 'Platform', users: [{ name: 'Alex Chen', email: 'alex@example.com' }], lastUpdated: new Date().toISOString() }],
          GetDisplayConfig: async () => ({
            visible_labels: [{ source: 'cluster' }, { source: 'namespace' }, { source: 'pod' }],
            visible_annotations: [{ source: 'summary' }, { source: 'description', style: ['pull'] }],
            subtitle_annotations: ['summary', 'description'],
            group_by: ['label:cluster', 'label:namespace'],
            group_by_override_key_mode: 'display',
            group_by_overrides: {},
            priority: { mode: 'before_sort', sources: [], source_types: [] },
            badges: [],
            sort_by: [{ field: 'field:severity', order: 'asc' }, { field: 'field:startsAt', order: 'desc' }],
          }),
          GetSeverityConfig: async () => ({
            default: 'unknown',
            levels: [
              { name: 'critical', color: '#ef4444', aliases: ['critical'] },
              { name: 'warning', color: '#f59e0b', aliases: ['warning'] },
              { name: 'info', color: '#3b82f6', aliases: ['info'] },
              { name: 'unknown', color: '#6b7280', aliases: ['unknown'] },
            ],
          }),
          GetSourceCapabilities: async () => ({ 'prod-am': { supportsSilence: true }, grafana: { supportsSilence: false } }),
          GetUIConfig: async () => (${JSON.stringify(uiConfig)}),
          GetUIScale: async () => ({ factor: 1, mode: 'fonts', apply_to_popup: true }),
          GetNotificationPermissionStatus: async () => 'authorized',
          GetActionsForAlert: async (id, source) => {
            const actions = ${JSON.stringify([])};
            const configured = ${JSON.stringify([{ Name: 'Open runbook', Match: {}, Action: { Type: 'url', Template: 'https://wiki.example/runbooks/disk' }, Icon: '' }])};
            return (id === '1' && source === 'prod-am') ? configured : actions;
          },
          ExecuteAction: async (name) => 'opened runbook URL',
          RefreshAlerts: async () => {},
          LayoutPopup: async () => {},
          GetAbout: async () => ({ name: 'Foghorn', version: 'screenshot', description: 'Demo', repoURL: 'https://github.com/sammy8806/foghorn', copyright: '' }),
        },
      },
    };
    window.runtime = {
      EventsOnMultiple: (eventName, callback) => {
        if (eventName === 'alerts:updated') {
          setTimeout(() => callback({ new: [], resolved: [], changed: [] }), 50);
        }
        return () => {};
      },
      EventsOn: (eventName, callback) => window.runtime.EventsOnMultiple(eventName, callback, -1),
      EventsOff: () => {},
      Environment: async () => ({ platform: 'linux', buildType: 'dev' }),
      BrowserOpenURL: async () => {},
    };
  `;
}
