// Silence permission decisions. One place for the rules that grey out silence
// controls: alert cards, groups, silence-from-search and the live editor check.
// The backend ships the raw permission state (SilenceAccess); this decides per
// silence. Unknown means enabled: only a successful lookup or a recorded
// refusal disables anything.
import type { Matcher, SilenceAccess, SourceCapabilities } from './alerts';

export const RULE_UID_LABEL = '__alert_rule_uid__';
export type SilenceOp = 'create' | 'write';
export interface SilenceDecision {
  allowed: boolean;
  reason: string; // '' when allowed
}

const ALLOWED: SilenceDecision = { allowed: true, reason: '' };
const MAX_ANCESTOR_STEPS = 64;

function refuse(reason: string): SilenceDecision {
  return { allowed: false, reason };
}

// ruleUID returns the value of an equality (non-regex) rule-UID matcher.
export function ruleUID(matchers: Matcher[]): string | null {
  const m = matchers.find(m => m.name === RULE_UID_LABEL && m.isEqual && !m.isRegex);
  return m ? m.value : null;
}

// folderPermitted walks from the rule's folder up to the root. It returns null
// when the walk reaches a folder missing from the folder list (the list failed
// to load, was truncated, or predates the folder): an ancestor grant can't be
// ruled out, and unknown means enabled.
function folderPermitted(folder: string, granted: string[], parents: Record<string, string>): boolean | null {
  const seen = new Set<string>();
  let current: string | undefined = folder;
  for (let step = 0; current && step < MAX_ANCESTOR_STEPS && !seen.has(current); step++) {
    if (granted.includes(current)) return true;
    if (!(current in parents)) return null;
    seen.add(current);
    current = parents[current];
  }
  return false;
}

export function decideSilence(
  source: string,
  access: SilenceAccess | undefined,
  matchers: Matcher[],
  op: SilenceOp,
): SilenceDecision {
  if (!access) return ALLOWED;
  const create = op === 'create';
  const ruleFolders = access.ruleFolders ?? {};
  const uid = ruleUID(matchers);
  const folder = uid !== null ? ruleFolders[uid] : undefined;

  // 1. Denials always apply.
  const d = access.denied;
  if (d) {
    const deniedFolders = (create ? d.createFolders : d.writeFolders) ?? [];
    const deniedHere =
      (create ? d.createSource : d.writeSource) ||
      (uid === null && (create ? d.createGeneral : d.writeGeneral)) ||
      (folder !== undefined && deniedFolders.includes(folder));
    if (deniedHere) {
      return refuse(`${source}: a silence was refused (HTTP ${d.status}); fix the credentials, then refresh`);
    }
  }

  // 2. Unknown means enabled.
  if (access.check !== 'ok' || access.granularity === 'off' || access.granularity === '') return ALLOWED;

  const all = create ? access.canCreateAll : access.canWriteAll;
  const granted = (create ? access.createFolders : access.writeFolders) ?? [];
  const sourceLevel = (): SilenceDecision =>
    all || granted.length > 0
      ? ALLOWED
      : refuse(create
        ? `${source}: token can't create silences (needs alert.silences:create)`
        : `${source}: token can't edit or expire silences (needs alert.silences:write)`);

  // 3. Granularity. A token without the permission anywhere gets the source
  // text even in folder mode: naming one folder would suggest the others work.
  if (access.granularity === 'source' || (!all && granted.length === 0)) return sourceLevel();
  if (uid === null) {
    if (all) return ALLOWED;
    return refuse(create
      ? `${source}: token can only silence single alert rules; keep the ${RULE_UID_LABEL} matcher (needs alert.silences:create on all folders)`
      : `${source}: token can only edit silences of single alert rules; keep the ${RULE_UID_LABEL} matcher (needs alert.silences:write on all folders)`);
  }
  if (folder === undefined) return sourceLevel();
  if (all || folderPermitted(folder, granted, access.folderParents ?? {}) !== false) return ALLOWED;
  const title = access.folderTitles?.[folder] || folder;
  return refuse(create
    ? `${source}: token can't silence alerts in folder "${title}" (needs alert.silences:create on that folder)`
    : `${source}: token can't edit or expire silences in folder "${title}" (needs alert.silences:write on that folder)`);
}

// silenceReadHint explains missing silence details on a "silenced" badge when
// a lookup showed the token can't read silences.
export function silenceReadHint(_source: string, access: SilenceAccess | undefined): string | null {
  if (access?.check === 'ok' && !access.canRead) {
    return "silence details hidden: token can't read silences (needs alert.silences:read)";
  }
  return null;
}

// decideAcrossSources is the decision for a silence that may go to any of the
// given sources (group without a fixed source, silence from search): allowed
// when one silence-capable source allows it, else refused with the reason of
// the first silence-capable source by name.
export function decideAcrossSources(
  capabilities: Record<string, SourceCapabilities>,
  sources: string[],
  matchers: Matcher[],
  op: SilenceOp,
): SilenceDecision {
  const capable = [...new Set(sources)].filter(s => capabilities[s]?.supportsSilence).sort();
  if (capable.length === 0) return ALLOWED;
  let first: SilenceDecision | null = null;
  for (const source of capable) {
    const decision = decideSilence(source, capabilities[source].silenceAccess, matchers, op);
    if (decision.allowed) return decision;
    first ??= decision;
  }
  return first ?? ALLOWED;
}
