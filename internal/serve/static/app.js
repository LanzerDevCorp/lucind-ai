'use strict';

const POLL_INTERVAL_MS = 2000;
const SDD_FLOW_FIELDS = Object.freeze([
  'run_id',
  'change',
  'sdd_phase',
  'fanout_group',
  'status',
  'lane_count',
  'lane_ids'
]);
const SDD_PLANNING_PHASES = Object.freeze([
  { key: 'explore', label: 'Explore' },
  { key: 'proposal', label: 'Proposal' },
  { key: 'spec', label: 'Spec' },
  { key: 'design', label: 'Design' },
  { key: 'tasks', label: 'Tasks' }
]);
const SDD_EXECUTION_PHASES = Object.freeze([
  { key: 'apply', label: 'Apply' },
  { key: 'verify', label: 'Verify' },
  { key: 'archive', label: 'Archive' }
]);
const SDD_PHASE_ALIASES = Object.freeze({ propose: 'proposal', specs: 'spec' });
const SDD_FLOW_STYLES = `
  .sdd-flows { display: grid; gap: 1.25rem; margin-top: 1.5rem; }
  .sdd-change { display: grid; gap: 1.25rem; }
  .sdd-change-title { margin-bottom: 0; font-family: var(--font-mono); overflow-wrap: anywhere; }
  .sdd-rail-section { display: grid; gap: 0.65rem; }
  .sdd-rail-title { margin: 0; color: var(--muted); font-family: var(--font-mono); font-size: 0.68rem; letter-spacing: 0.12em; text-transform: uppercase; }
  .sdd-rail { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 0.65rem; }
  .sdd-phase { min-width: 0; padding: 0.85rem; border: 1px solid var(--line); background: var(--surface-deep); }
  .sdd-phase h4 { margin: 0 0 0.7rem; font-family: var(--font-mono); }
  .sdd-flow-group { display: grid; gap: 0.4rem; margin-top: 0.55rem; padding: 0.65rem; border-left: 4px solid var(--line-strong); background: var(--surface); }
  .sdd-flow-group[data-flow-role="planning-lenses"] { border-left-color: var(--warning); }
  .sdd-flow-group[data-flow-role="synthesis"] { border-left-color: var(--signal); }
  .sdd-flow-group[data-flow-role="execution"] { border-left-color: var(--live); }
  .sdd-flow-role { color: var(--ink); font-weight: 700; }
  .sdd-flow-meta, .sdd-flow-lanes, .sdd-phase-missing { margin: 0; color: var(--muted); font-family: var(--font-mono); font-size: 0.72rem; line-height: 1.5; }
  .sdd-flow-lanes { display: grid; gap: 0.25rem; padding: 0; list-style: none; }
`;

function createLiveStore({
  fetchStateImpl = window.fetch.bind(window),
  EventSourceImpl = window.EventSource,
  setIntervalImpl = window.setInterval.bind(window),
  clearIntervalImpl = window.clearInterval.bind(window)
} = {}) {
  let cachedState = null;
  let connectionMetadata = {
    mode: 'connecting',
    message: 'Connecting to live updates…',
    error: null
  };
  let eventSource = null;
  let pollingTimer = null;
  let refreshInFlight = false;
  let stopped = false;
  const subscribers = new Set();

  function getSnapshot() {
    return {
      state: cachedState,
      connection: { ...connectionMetadata }
    };
  }

  function notify() {
    const snapshot = getSnapshot();
    subscribers.forEach(subscriber => subscriber(snapshot));
  }

  function setConnection(mode, message, error = null) {
    connectionMetadata = { mode, message, error };
    notify();
  }

  async function refreshState(source = 'manual') {
    if (stopped || refreshInFlight) return false;
    refreshInFlight = true;

    try {
      const response = await fetchStateImpl('/api/state', {
        headers: { Accept: 'application/json' },
        cache: 'no-store'
      });
      if (!response.ok) {
        throw new Error(`State request failed with HTTP ${response.status}`);
      }

      const nextState = await response.json();
      cachedState = nextState;
      if (source === 'poll') {
        connectionMetadata = {
          mode: 'polling',
          message: 'Stream unavailable; polling every 2 seconds.',
          error: null
        };
      }
      notify();
      return true;
    } catch (error) {
      const cacheMessage = cachedState === null
        ? 'no state has loaded yet.'
        : 'showing cached data.';
      setConnection('error', `State refresh failed; ${cacheMessage}`, String(error));
      return false;
    } finally {
      refreshInFlight = false;
    }
  }

  function closeStream() {
    if (eventSource === null) return;
    eventSource.close();
    eventSource = null;
  }

  function stopPolling() {
    if (pollingTimer === null) return;
    clearIntervalImpl(pollingTimer);
    pollingTimer = null;
  }

  function startPolling(reason) {
    if (stopped) return;
    if (pollingTimer !== null) return;
    closeStream();
    setConnection('polling', `${reason}; polling every 2 seconds.`);
    void refreshState('poll');
    pollingTimer = setIntervalImpl(() => {
      void refreshState('poll');
    }, POLL_INTERVAL_MS);
  }

  function connectSSE() {
    if (stopped) return;
    if (typeof EventSourceImpl !== 'function') {
      startPolling('Event stream is not supported');
      return;
    }

    eventSource = new EventSourceImpl('/api/stream');
    eventSource.addEventListener('open', () => {
      stopPolling();
      setConnection('live', 'Live event stream connected.');
      void refreshState('stream-open');
    });

    const refreshFromStream = () => {
      void refreshState('stream');
    };
    eventSource.addEventListener('event', refreshFromStream);
    eventSource.addEventListener('progress', refreshFromStream);
    eventSource.addEventListener('resync', () => {
      startPolling('Stream requested resynchronization');
    });
    eventSource.addEventListener('error', () => {
      startPolling('Stream unavailable');
    });
  }

  function subscribe(subscriber) {
    subscribers.add(subscriber);
    subscriber(getSnapshot());
    return () => subscribers.delete(subscriber);
  }

  function start() {
    connectSSE();
    void refreshState('initial');
  }

  function teardown() {
    stopped = true;
    closeStream();
    stopPolling();
    subscribers.clear();
  }

  return { getSnapshot, refreshState, start, subscribe, teardown };
}

function isValidEvidence(ev) {
  if (!ev || typeof ev !== 'string') return false;
  const trimmed = ev.trim();
  if (!trimmed) return false;
  const hasFileLine = /[\w\-./]+\.\w+:\d+/.test(trimmed);
  const hasCommandOutput = trimmed.startsWith('ok ') ||
    trimmed.startsWith('PASS') ||
    trimmed.startsWith('$ ') ||
    trimmed.includes('---');
  return hasFileLine || hasCommandOutput;
}

function normalizeApproval(item) {
  const runID = String(field(item, 'run_id', 'RunID') || '');
  const laneID = String(field(item, 'lane_id', 'LaneID') || '');
  const packetID = String(field(item, 'packet_id', 'PacketID') || '');
  const approver = String(field(item, 'approver', 'Approver') || '');
  const evidence = String(field(item, 'evidence', 'Evidence') || '');
  const decision = String(field(item, 'decision', 'Decision') || 'pending').toLowerCase();
  const defect = Boolean(field(item, 'defect_surfaced_later', 'DefectSurfacedLater', 'defect', 'Defect'));
  const requestedAt = field(item, 'requested_at', 'RequestedAt');
  const decidedAt = field(item, 'decided_at', 'DecidedAt');
  return {
    runID,
    laneID,
    packetID,
    approver,
    evidence,
    decision,
    defect,
    requestedAt,
    decidedAt,
    key: `${runID}\u0000${laneID}`
  };
}

function normalizeApprovals(state) {
  const raw = [
    ...asArray(state, 'approvals', 'Approvals'),
    ...asArray(state, 'pending_approvals_list', 'PendingApprovalsList')
  ];
  return raw.map(normalizeApproval).filter(item => Boolean(item.runID && item.laneID));
}

function approvalKey(item) {
  const runID = field(item, 'run_id', 'RunID') || item.runID || '';
  const laneID = field(item, 'lane_id', 'LaneID') || item.laneID || '';
  return `${runID}\u0000${laneID}`;
}

function createApprovalCard(rawItem) {
  const item = normalizeApproval(rawItem);
  const card = document.createElement('article');
  card.className = 'approval-card';
  card.setAttribute('data-approval-key', item.key);

  const header = document.createElement('div');
  header.className = 'card-header';
  const headerInfo = document.createElement('div');
  headerInfo.className = 'card-header-info';
  const lane = document.createElement('span');
  lane.className = 'lane-id';
  const packet = document.createElement('span');
  packet.className = 'packet-id';
  headerInfo.append(lane, packet);

  const badges = document.createElement('div');
  badges.className = 'approval-badges';
  const decisionBadge = document.createElement('span');
  decisionBadge.className = 'badge badge-decision';
  const defectBadge = document.createElement('span');
  defectBadge.className = 'badge badge-defect';
  defectBadge.textContent = 'Defect surfaced';
  badges.append(decisionBadge, defectBadge);
  header.append(headerInfo, badges);

  const evidenceLabel = document.createElement('div');
  evidenceLabel.className = 'evidence-label';
  evidenceLabel.textContent = 'Approval evidence';
  const evidence = document.createElement('pre');
  evidence.className = 'evidence-block';
  const noEvidence = document.createElement('div');
  noEvidence.className = 'no-evidence';
  noEvidence.textContent = 'No command output or file:line evidence provided.';

  const meta = document.createElement('div');
  meta.className = 'approval-meta';
  const approverInfo = document.createElement('span');
  approverInfo.className = 'approver-info';
  const decidedInfo = document.createElement('span');
  decidedInfo.className = 'decided-info';
  meta.append(approverInfo, decidedInfo);

  const actions = document.createElement('div');
  actions.className = 'card-actions';
  const approveButton = document.createElement('button');
  approveButton.type = 'button';
  approveButton.className = 'btn-approve';
  approveButton.textContent = 'Approve';
  const rejectButton = document.createElement('button');
  rejectButton.type = 'button';
  rejectButton.className = 'btn-reject';
  rejectButton.textContent = 'Reject';
  const defectButton = document.createElement('button');
  defectButton.type = 'button';
  defectButton.className = 'btn-defect';
  defectButton.textContent = 'Mark defect';
  actions.append(approveButton, rejectButton, defectButton);

  async function decide(decision) {
    approveButton.disabled = true;
    rejectButton.disabled = true;
    const accepted = await submitDecision(card._approvalItem.runID, card._approvalItem.laneID, decision);
    if (!accepted) {
      approveButton.disabled = false;
      rejectButton.disabled = false;
    }
  }

  async function toggleDefect() {
    defectButton.disabled = true;
    const nextDefect = !card._approvalItem.defect;
    const accepted = await submitDefect(card._approvalItem.runID, card._approvalItem.laneID, nextDefect);
    if (!accepted) {
      defectButton.disabled = false;
    }
  }

  approveButton.addEventListener('click', () => void decide('approved'));
  rejectButton.addEventListener('click', () => void decide('rejected'));
  defectButton.addEventListener('click', () => void toggleDefect());

  card.append(header, evidenceLabel, evidence, noEvidence, meta, actions);
  card._approvalParts = {
    lane,
    packet,
    decisionBadge,
    defectBadge,
    evidence,
    noEvidence,
    meta,
    approverInfo,
    decidedInfo,
    approveButton,
    rejectButton,
    defectButton
  };
  updateApprovalCard(card, item);
  return card;
}

function updateApprovalCard(card, rawItem) {
  const item = normalizeApproval(rawItem);
  card._approvalItem = item;
  const parts = card._approvalParts;

  card.setAttribute('data-decision', item.decision);
  card.setAttribute('data-status', item.decision);
  card.setAttribute('data-defect', String(item.defect));
  card.dataset.decision = item.decision;
  card.dataset.status = item.decision;
  card.dataset.defect = String(item.defect);

  parts.lane.textContent = `Lane: ${item.laneID || '-'}`;
  parts.packet.textContent = `Packet: ${item.packetID || '-'}`;

  const isPending = item.decision === 'pending';
  parts.decisionBadge.textContent = item.decision;
  parts.decisionBadge.setAttribute('data-decision', item.decision);
  parts.decisionBadge.dataset.decision = item.decision;
  parts.defectBadge.setAttribute('data-defect', String(item.defect));
  parts.defectBadge.hidden = !item.defect;

  const evidenceValid = isValidEvidence(item.evidence);
  parts.evidence.textContent = evidenceValid ? item.evidence : '';
  parts.evidence.hidden = !evidenceValid;
  parts.noEvidence.hidden = evidenceValid;

  if (item.approver || item.decidedAt) {
    parts.meta.hidden = false;
    parts.approverInfo.textContent = item.approver ? `Approver: ${item.approver}` : '';
    parts.decidedInfo.textContent = item.decidedAt ? `Decided: ${item.decidedAt}` : '';
  } else {
    parts.meta.hidden = isPending;
  }

  if (isPending) {
    parts.approveButton.disabled = false;
    parts.rejectButton.disabled = false;
    parts.approveButton.hidden = false;
    parts.rejectButton.hidden = false;
  } else {
    parts.approveButton.disabled = true;
    parts.rejectButton.disabled = true;
  }

  parts.defectButton.textContent = item.defect ? 'Unmark defect' : 'Mark defect';
  parts.defectButton.dataset.active = String(item.defect);
  parts.defectButton.disabled = false;
}

function patchApprovalCards(approvalsContainer, rawApprovals) {
  const approvals = Array.isArray(rawApprovals) ? rawApprovals.map(normalizeApproval) : [];
  const currentCards = new Map();
  approvalsContainer.querySelectorAll('[data-approval-key]').forEach(card => {
    currentCards.set(card.getAttribute('data-approval-key'), card);
  });

  const activeKeys = new Set();
  approvals.forEach(item => {
    activeKeys.add(item.key);
    const card = currentCards.get(item.key);
    if (card) {
      updateApprovalCard(card, item);
    } else {
      approvalsContainer.appendChild(createApprovalCard(item));
    }
  });

  currentCards.forEach((card, key) => {
    if (!activeKeys.has(key)) card.remove();
  });

  let emptyState = approvalsContainer.querySelector('[data-empty-state]');
  if (approvals.length === 0 && emptyState === null) {
    emptyState = document.createElement('div');
    emptyState.className = 'empty-state';
    emptyState.setAttribute('data-empty-state', '');
    emptyState.textContent = 'No pending approvals.';
    approvalsContainer.appendChild(emptyState);
  } else if (approvals.length > 0 && emptyState !== null) {
    emptyState.remove();
  }

  const pendingCount = approvals.filter(item => item.decision === 'pending').length;
  return pendingCount;
}

const FLEET_STATUS = {
  pending: ['◇', 'Pending'],
  running: ['▶', 'Running'],
  done: ['◆', 'Done'],
  blocked: ['■', 'Blocked'],
  deviated: ['△', 'Deviated'],
  failed: ['✕', 'Failed']
};

function injectFleetStyles() {
  if (document.getElementById('fleet-styles')) return;
  const style = document.createElement('style');
  style.id = 'fleet-styles';
  style.textContent = `
    .activity-card.fleet-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(310px,1fr)); gap:1rem; padding:0; border:0; background:transparent; color:var(--ink); }
    .fleet-card { min-width:0; padding:1rem; border:1px solid var(--line); background:var(--surface-raised); box-shadow:7px 7px 0 rgba(0,0,0,.2); }
    .fleet-card[data-status="running"] { border-left:5px solid var(--live); }
    .fleet-card[data-status="blocked"], .fleet-card[data-status="failed"] { border-left:5px solid var(--danger); }
    .fleet-card[data-status="pending"], .fleet-card[data-status="deviated"] { border-left:5px solid var(--warning); }
    .fleet-card[data-status="done"] { border-left:5px solid var(--signal); }
    .fleet-header { display:flex; justify-content:space-between; gap:1rem; padding-bottom:.75rem; border-bottom:1px solid var(--line); }
    .fleet-status { flex:0 0 auto; font-family:var(--font-mono); font-weight:800; }
    .fleet-status-symbol { margin-right:.35rem; }
    .fleet-fields { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.75rem 1rem; margin:.9rem 0; }
    .fleet-field { min-width:0; }
    .fleet-label { display:block; color:var(--muted); font-family:var(--font-mono); font-size:.62rem; letter-spacing:.08em; text-transform:uppercase; }
    .fleet-value { display:block; overflow-wrap:anywhere; margin-top:.2rem; font-family:var(--font-mono); font-size:.78rem; }
    .fleet-activity { min-height:4.4rem; padding:.75rem; border:1px solid var(--line); background:var(--surface-deep); }
    .fleet-activity .fleet-value { white-space:pre-wrap; }
    .fleet-indicators { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); margin-top:.8rem; border:1px solid var(--line); }
    .fleet-indicator { min-width:0; padding:.6rem; border-right:1px solid var(--line); }
    .fleet-indicator:last-child { border-right:0; }
    .fleet-empty { grid-column:1/-1; padding:2.4rem; border:1px dashed var(--line-strong); background:var(--surface); color:var(--muted); }
    @media (max-width:600px) { .fleet-fields { grid-template-columns:1fr; } .fleet-indicators { grid-template-columns:1fr; } .fleet-indicator { border-right:0; border-bottom:1px solid var(--line); } }
  `;
  document.head.appendChild(style);
}

function field(source, ...names) {
  if (!source || typeof source !== 'object') return undefined;
  for (const name of names) {
    if (source[name] !== undefined && source[name] !== null && source[name] !== '') return source[name];
  }
  return undefined;
}

function asArray(source, ...names) {
  const value = field(source, ...names);
  return Array.isArray(value) ? value : [];
}

function latestProgress(items) {
  return items.reduce((latest, item) => {
    if (!latest) return item;
    const itemSeq = Number(field(item, 'seq', 'Seq'));
    const latestSeq = Number(field(latest, 'seq', 'Seq'));
    if (Number.isFinite(itemSeq) && Number.isFinite(latestSeq)) return itemSeq > latestSeq ? item : latest;
    return Date.parse(field(item, 'at', 'At') || 0) > Date.parse(field(latest, 'at', 'At') || 0) ? item : latest;
  }, null);
}

function displayMetric(value, formatter = String) {
  return value === undefined || value === null || value === '' ? 'Unavailable' : formatter(value);
}

function formatElapsed(startedAt, endedAt, now = Date.now()) {
  const start = Date.parse(startedAt);
  const end = endedAt ? Date.parse(endedAt) : now;
  if (!Number.isFinite(start) || !Number.isFinite(end)) return 'Unavailable';
  const totalSeconds = Math.max(0, Math.floor((end - start) / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return hours > 0 ? `${hours}h ${minutes}m ${seconds}s` : `${minutes}m ${seconds}s`;
}

function normalizeFleetState(state, now = Date.now()) {
  const runs = asArray(state, 'runs', 'Runs');
  const topLanes = asArray(state, 'lanes', 'Lanes');
  const topProgress = asArray(state, 'progress', 'Progress', 'lane_progress', 'LaneProgress');
  const runsByID = new Map(runs.map(run => [String(field(run, 'run_id', 'RunID', 'id', 'ID') || ''), run]));
  const candidates = [];
  runs.forEach(run => asArray(run, 'lanes', 'Lanes').forEach(lane => candidates.push([lane, run])));
  topLanes.forEach(lane => candidates.push([lane, runsByID.get(String(field(lane, 'run_id', 'RunID') || '')) || {}]));

  const normalized = new Map();
  candidates.forEach(([lane, run], index) => {
    const runID = String(field(lane, 'run_id', 'RunID') || field(run, 'run_id', 'RunID', 'id', 'ID') || '-');
    const laneID = String(field(lane, 'lane_id', 'LaneID', 'id', 'ID', 'packet_id', 'PacketID') || `lane-${index + 1}`);
    const progress = [
      ...topProgress,
      ...asArray(run, 'progress', 'Progress', 'lane_progress', 'LaneProgress'),
      ...asArray(lane, 'progress', 'Progress', 'lane_progress', 'LaneProgress')
    ].filter(item => {
      const itemRunID = field(item, 'run_id', 'RunID');
      const itemLaneID = field(item, 'lane_id', 'LaneID');
      return (!itemRunID || String(itemRunID) === runID) && (!itemLaneID || String(itemLaneID) === laneID);
    });
    const latest = field(lane, 'latest_progress', 'LatestProgress') || latestProgress(progress) || {};
    const telemetry = field(lane, 'telemetry', 'Telemetry', 'metrics', 'Metrics', 'usage', 'Usage') ||
      field(latest, 'telemetry', 'Telemetry', 'metrics', 'Metrics', 'usage', 'Usage') || latest;
    const status = String(field(lane, 'status', 'Status') || 'pending').toLowerCase();
    const activity = field(latest, 'message', 'Message', 'activity', 'Activity') ||
      field(lane, 'latest_activity', 'LatestActivity', 'note', 'Note') || `Lane is ${status}.`;
    const startedAt = field(lane, 'started_at', 'StartedAt') || field(run, 'started_at', 'StartedAt');
    const endedAt = field(lane, 'ended_at', 'EndedAt') || field(run, 'ended_at', 'EndedAt');
    normalized.set(`${runID}\u0000${laneID}`, {
      key: `${runID}\u0000${laneID}`,
      laneID,
      executor: field(lane, 'executor', 'Executor') || 'Unavailable',
      model: field(lane, 'model', 'Model') || 'Unavailable',
      phase: field(lane, 'sdd_phase', 'SDDPhase') || field(run, 'sdd_phase', 'SDDPhase') || 'Unavailable',
      fanout: field(lane, 'fanout_group', 'FanoutGroup') || field(run, 'fanout_group', 'FanoutGroup') || 'Unavailable',
      feature: field(lane, 'feature', 'Feature', 'feature_id', 'FeatureID') || field(run, 'feature', 'Feature', 'feature_id', 'FeatureID') || 'Unavailable',
      worktree: field(lane, 'worktree_path', 'WorktreePath', 'worktree', 'Worktree') || 'Unavailable',
      attempt: field(lane, 'attempt', 'Attempt') || 'Unavailable',
      elapsed: formatElapsed(startedAt, endedAt, now),
      status,
      activity: String(activity),
      tokens: displayMetric(field(telemetry, 'total_tokens', 'TotalTokens', 'tokens', 'Tokens')),
      cost: displayMetric(field(telemetry, 'cost_usd', 'CostUSD', 'cost', 'Cost'), value => `$${Number(value).toFixed(4)}`),
      toolRate: displayMetric(field(telemetry, 'tool_rate', 'ToolRate', 'tools_per_minute', 'ToolsPerMinute'), value => `${value}/min`)
    });
  });
  return Array.from(normalized.values());
}

function makeFleetPart(tag, className, parent) {
  const part = document.createElement(tag);
  part.className = className;
  if (parent) parent.appendChild(part);
  return part;
}

function makeFleetField(parent, label, className = 'fleet-field') {
  const wrapper = makeFleetPart('div', className, parent);
  const labelElement = makeFleetPart('span', 'fleet-label', wrapper);
  labelElement.textContent = label;
  return makeFleetPart('span', 'fleet-value', wrapper);
}

function createFleetCard(lane) {
  const card = makeFleetPart('article', 'fleet-card');
  card.setAttribute('data-fleet-key', lane.key);
  const header = makeFleetPart('div', 'fleet-header', card);
  const laneID = makeFleetPart('span', 'lane-id', header);
  const status = makeFleetPart('span', 'fleet-status', header);
  const statusSymbol = makeFleetPart('span', 'fleet-status-symbol', status);
  statusSymbol.setAttribute('aria-hidden', 'true');
  const statusText = makeFleetPart('span', 'fleet-status-text', status);
  const fields = makeFleetPart('div', 'fleet-fields', card);
  const parts = { laneID, statusSymbol, statusText };
  [['executor', 'Executor'], ['model', 'Model'], ['phase', 'SDD phase'], ['fanout', 'Fanout group'], ['feature', 'Feature'], ['worktree', 'Worktree'], ['attempt', 'Attempt'], ['elapsed', 'Elapsed']]
    .forEach(([name, label]) => { parts[name] = makeFleetField(fields, label); });
  parts.activity = makeFleetField(card, 'Latest activity', 'fleet-activity');
  const indicators = makeFleetPart('div', 'fleet-indicators', card);
  [['tokens', 'Tokens'], ['cost', 'Cost'], ['toolRate', 'Tool rate']]
    .forEach(([name, label]) => { parts[name] = makeFleetField(indicators, label, 'fleet-indicator'); });
  card._fleetParts = parts;
  updateFleetCard(card, lane);
  return card;
}

function updateFleetCard(card, lane) {
  const parts = card._fleetParts;
  const [symbol, label] = FLEET_STATUS[lane.status] || ['?', lane.status || 'Unknown'];
  card.dataset.status = lane.status;
  parts.laneID.textContent = `Lane: ${lane.laneID}`;
  parts.statusSymbol.textContent = symbol;
  parts.statusText.textContent = label;
  ['executor', 'model', 'phase', 'fanout', 'feature', 'worktree', 'attempt', 'elapsed', 'activity', 'tokens', 'cost', 'toolRate']
    .forEach(name => { parts[name].textContent = lane[name]; });
}

function patchFleetCards(container, lanes) {
  injectFleetStyles();
  container.classList.add('fleet-grid');
  Array.from(container.childNodes).filter(node => node.nodeType === Node.TEXT_NODE).forEach(node => node.remove());
  const current = new Map();
  container.querySelectorAll('[data-fleet-key]').forEach(card => current.set(card.getAttribute('data-fleet-key'), card));
  const active = new Set();
  lanes.forEach(lane => {
    active.add(lane.key);
    const card = current.get(lane.key);
    if (card) updateFleetCard(card, lane);
    else container.appendChild(createFleetCard(lane));
  });
  current.forEach((card, key) => { if (!active.has(key)) card.remove(); });
  let empty = container.querySelector('[data-fleet-empty]');
  if (lanes.length === 0 && !empty) {
    empty = makeFleetPart('div', 'fleet-empty', container);
    empty.setAttribute('data-fleet-empty', '');
    empty.textContent = 'No lanes are reporting yet.';
  } else if (lanes.length > 0 && empty) empty.remove();
}

function renderConnection(connection) {
  const status = document.getElementById('connection-status');
  const text = document.getElementById('connection-status-text');
  if (!status || !text) return;
  status.dataset.mode = connection.mode;
  text.textContent = connection.message;
}

const APPLY_DAG_STYLE_ID = 'apply-dag-view-styles';

function ensureApplyDAGStyles() {
  if (document.getElementById(APPLY_DAG_STYLE_ID)) return;
  const style = document.createElement('style');
  style.id = APPLY_DAG_STYLE_ID;
  style.textContent = `
    .apply-dag-view { display: grid; gap: 1rem; margin-top: 1rem; }
    .dag-wave, .dag-dependencies, .dag-overlaps { padding: 1rem; border: 1px solid var(--line); background: var(--surface); }
    .dag-packets { display: grid; gap: 0.65rem; }
    .dag-packet { padding: 0.8rem; border: 1px solid var(--line-strong); background: var(--surface-raised); }
    .dag-packet [data-packet-status] { color: var(--muted); font-family: var(--font-mono); }
    .dag-packet[data-status="pending"] { border-left: 4px solid var(--muted); }
    .dag-packet[data-status="running"] { border-left: 4px solid var(--warning); }
    .dag-packet[data-status="done"] { border-left: 4px solid var(--live); }
    .dag-packet[data-status="blocked"] { border-left: 4px solid var(--danger); }
    .dag-packet[data-status="deviated"] { border-left: 4px solid var(--warning); }
    .dag-packet[data-status="failed"] { border-left: 4px solid var(--danger); }
    .dag-dependency { font-family: var(--font-mono); }
    .dag-overlap-error { padding: 0.8rem; border: 1px solid var(--danger); background: rgba(255, 90, 90, 0.08); color: var(--danger); white-space: pre-wrap; }
  `;
  document.head.appendChild(style);
}

function createApplyDAGRegion() {
  const outlet = document.getElementById('activity-view');
  if (!outlet) return null;
  ensureApplyDAGStyles();

  const region = document.createElement('section');
  region.id = 'apply-dag-view';
  region.className = 'apply-dag-view';
  region.setAttribute('aria-label', 'Apply DAG execution');
  region.setAttribute('aria-live', 'polite');
  outlet.appendChild(region);
  return region;
}

function renderApplyDAG(dag) {
  let region = document.getElementById('apply-dag-view');
  if (!dag) {
    if (region) region.hidden = true;
    return;
  }
  if (!region) region = createApplyDAGRegion();
  if (!region) return;

  region.hidden = false;
  const fragment = document.createDocumentFragment();

  const heading = document.createElement('h2');
  heading.textContent = `Apply DAG · ${dag.change}`;
  fragment.appendChild(heading);

  dag.waves.forEach(wave => {
    const waveSection = document.createElement('section');
    waveSection.className = 'dag-wave';
    waveSection.setAttribute('data-wave-number', String(wave.number));

    const waveHeading = document.createElement('h3');
    waveHeading.textContent = `Wave ${wave.number}`;
    waveSection.appendChild(waveHeading);

    const packetList = document.createElement('div');
    packetList.className = 'dag-packets';
    wave.packets.forEach(packet => {
      const packetNode = document.createElement('article');
      packetNode.className = 'dag-packet';
      packetNode.setAttribute('data-packet-id', packet.id);
      packetNode.dataset.status = packet.status;

      const packetID = document.createElement('strong');
      packetID.textContent = packet.id;
      const packetStatus = document.createElement('span');
      packetStatus.setAttribute('data-packet-status', '');
      packetStatus.textContent = ` ${packet.status}`;
      packetNode.append(packetID, packetStatus);
      packetList.appendChild(packetNode);
    });
    waveSection.appendChild(packetList);
    fragment.appendChild(waveSection);
  });

  const dependencies = document.createElement('section');
  dependencies.className = 'dag-dependencies';
  const dependencyHeading = document.createElement('h3');
  dependencyHeading.textContent = 'Dependencies';
  dependencies.appendChild(dependencyHeading);
  dag.dependencies.forEach(dependency => {
    const edge = document.createElement('div');
    edge.className = 'dag-dependency';
    edge.textContent = `${dependency.from} → ${dependency.to}`;
    dependencies.appendChild(edge);
  });
  fragment.appendChild(dependencies);

  const overlaps = document.createElement('section');
  overlaps.className = 'dag-overlaps';
  const overlapHeading = document.createElement('h3');
  overlapHeading.textContent = 'Overlap violations';
  overlaps.appendChild(overlapHeading);
  dag.overlap_violations.forEach(violation => {
    const error = document.createElement('pre');
    error.className = 'dag-overlap-error';
    error.textContent = JSON.stringify(violation, null, 2);
    overlaps.appendChild(error);
  });
  fragment.appendChild(overlaps);

  region.replaceChildren(fragment);
}

const FEATURE_SWIMLANE_STYLES = `
  .feature-swimlanes { display: grid; gap: 1.25rem; margin-top: 1.5rem; }
  .feature-swimlane { min-width: 0; padding: 1.25rem; border: 1px solid var(--line); background: var(--surface-raised); box-shadow: 7px 7px 0 rgba(0, 0, 0, 0.2); }
  .feature-swimlane[data-status="active"] { border-left: 5px solid var(--live); }
  .feature-swimlane[data-status="expired"] { border-left: 5px solid var(--warning); }
  .feature-swimlane[data-status="blocked"] { border-left: 5px solid var(--danger); }
  .feature-swimlane[data-status="promoted"] { border-left: 5px solid var(--signal); }
  .feature-swimlane[data-status="reconciliation-required"] { border-left: 5px solid var(--warning); }
  .swimlane-header { display: flex; justify-content: space-between; align-items: center; gap: 1rem; padding-bottom: 0.75rem; border-bottom: 1px solid var(--line); flex-wrap: wrap; }
  .swimlane-title-group { display: flex; align-items: center; gap: 0.65rem; }
  .feature-id { font-family: var(--font-mono); font-weight: 700; font-size: 1.05rem; }
  .feature-badges { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: center; }
  .badge { padding: 0.25rem 0.55rem; border: 1px solid var(--line-strong); font-family: var(--font-mono); font-size: 0.68rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.06em; background: var(--surface); }
  .badge-feature-status { color: var(--ink); border-color: var(--line); }
  .badge-reconcile[data-reconcile-badge="required"] { background: rgba(255, 198, 92, 0.15); color: var(--warning); border-color: var(--warning); }
  .badge-lease[data-lease-status="active"] { color: var(--live); border-color: var(--live); }
  .badge-lease[data-lease-status="expired"] { color: var(--warning); border-color: var(--warning); }
  .badge-promoted { color: var(--signal); border-color: var(--signal); }
  .badge-blocked { color: var(--danger); border-color: var(--danger); }
  .badge-expired { color: var(--warning); border-color: var(--warning); }
  .swimlane-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1rem; margin-top: 1rem; }
  .swimlane-section { min-width: 0; padding: 0.85rem; border: 1px solid var(--line); background: var(--surface-deep); display: grid; gap: 0.6rem; align-content: start; }
  .swimlane-section-title { margin: 0 0 0.35rem; color: var(--muted); font-family: var(--font-mono); font-size: 0.65rem; letter-spacing: 0.1em; text-transform: uppercase; }
  .ref-item, .lease-item, .recon-field { display: flex; justify-content: space-between; gap: 0.5rem; font-family: var(--font-mono); font-size: 0.76rem; overflow-wrap: anywhere; }
  .ref-label, .lease-label, .recon-label, .attempt-label { color: var(--muted); font-size: 0.68rem; text-transform: uppercase; }
  .ref-value, .lease-value, .recon-value, .attempt-value { color: var(--ink); font-weight: 600; text-align: right; }
  .attempt-card, .overlap-card, .reconcile-card { padding: 0.65rem; border: 1px solid var(--line); background: var(--surface); display: grid; gap: 0.4rem; }
  .attempt-card[data-status="promoted"] { border-left: 3px solid var(--signal); }
  .attempt-card[data-status="blocked"] { border-left: 3px solid var(--danger); }
  .attempt-card[data-status="failed"] { border-left: 3px solid var(--danger); }
  .attempt-header, .overlap-header, .reconcile-header { display: flex; justify-content: space-between; align-items: center; gap: 0.5rem; font-family: var(--font-mono); font-size: 0.75rem; }
  .overlap-evidence-json, .failure-reason { margin: 0.4rem 0 0; padding: 0.5rem; border: 1px solid var(--line); background: var(--surface-deep); color: #d7dcd7; font-family: var(--font-mono); font-size: 0.72rem; line-height: 1.45; white-space: pre-wrap; overflow-wrap: anywhere; }
  .evidence-class-badge { font-family: var(--font-mono); font-size: 0.68rem; font-weight: 700; text-transform: uppercase; color: var(--warning); }
  .evidence-class-badge[data-class="required"] { color: var(--danger); }
  .evidence-class-badge[data-class="warning"] { color: var(--warning); }
  .evidence-class-badge[data-class="informational"] { color: var(--live); }
  .empty-note { color: var(--muted); font-family: var(--font-mono); font-size: 0.72rem; font-style: italic; }
  .features-empty { padding: 2.4rem; border: 1px dashed var(--line-strong); background: var(--surface); color: var(--muted); }
`;

function ensureFeatureSwimlanesContainer() {
  let container = document.getElementById('feature-swimlanes-container');
  if (container) return container;

  const activityView = document.getElementById('activity-view');
  if (!activityView) return null;

  if (!document.getElementById('feature-swimlane-styles')) {
    const style = document.createElement('style');
    style.id = 'feature-swimlane-styles';
    style.textContent = FEATURE_SWIMLANE_STYLES;
    document.head.appendChild(style);
  }

  container = document.createElement('section');
  container.id = 'feature-swimlanes-container';
  container.className = 'feature-swimlanes';
  container.setAttribute('aria-labelledby', 'feature-swimlanes-title');
  container.appendChild(createTextElement('h2', 'sdd-change-title', 'Feature swimlanes'));
  container.firstChild.id = 'feature-swimlanes-title';
  activityView.appendChild(container);
  return container;
}

// Lease countdowns are differences against an absolute expires_at, so they are
// only as trustworthy as the clock they are differenced from. A viewer whose
// machine is a few minutes fast reads a live lease as expired; a few minutes
// slow and a dead one still looks held. Neither is acceptable in a console
// whose job is to show real lease state.
//
// Every state payload carries the server's own clock as server_time. We measure
// the offset once per payload and hand serverNow() to the normalizers instead of
// Date.now(), so every countdown on screen is anchored to the server rather than
// to whatever the browser believes. The offset is a single measurement, not a
// running sync: no extra endpoint, no polling for time.
let serverClockOffsetMs = 0;

function syncServerClock(state, now = Date.now()) {
  const raw = state && (state.server_time || state.ServerTime);
  if (!raw) return serverClockOffsetMs;
  const parsed = Date.parse(raw);
  if (Number.isNaN(parsed)) return serverClockOffsetMs;
  serverClockOffsetMs = parsed - now;
  return serverClockOffsetMs;
}

function serverNow(now = Date.now()) {
  return now + serverClockOffsetMs;
}

function formatTTL(expiresAt, now = Date.now()) {
  if (!expiresAt) return 'Unavailable';
  const expiry = Date.parse(expiresAt);
  if (!Number.isFinite(expiry)) return 'Unavailable';
  const remainingMs = expiry - now;
  if (remainingMs <= 0) return 'Expired';
  const totalSeconds = Math.floor(remainingMs / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
  if (minutes > 0) return `${minutes}m ${seconds}s`;
  return `${seconds}s`;
}

function getLeaseStatus(expiresAt, now = Date.now()) {
  if (!expiresAt) return 'none';
  const expiry = Date.parse(expiresAt);
  if (!Number.isFinite(expiry)) return 'none';
  return expiry <= now ? 'expired' : 'active';
}

function normalizeFeatureSwimlanes(state, now = Date.now()) {
  const rawFeatures = [
    ...asArray(state, 'features', 'Features'),
    ...asArray(state, 'feature_swimlanes', 'FeatureSwimlanes', 'swimlanes', 'Swimlanes')
  ];
  const allLeases = [
    ...asArray(state, 'leases', 'Leases'),
    ...asArray(state, 'feature_leases', 'FeatureLeases')
  ];
  const allAttempts = [
    ...asArray(state, 'attempts', 'Attempts'),
    ...asArray(state, 'integration_attempts', 'IntegrationAttempts')
  ];
  const allOverlaps = [
    ...asArray(state, 'overlap_evidence', 'OverlapEvidence'),
    ...asArray(state, 'overlaps', 'Overlaps')
  ];
  const allReconciliations = [
    ...asArray(state, 'reconciliations', 'Reconciliations'),
    ...asArray(state, 'reconciliation_requests', 'ReconciliationRequests')
  ];

  const leasesByFeature = new Map();
  allLeases.forEach(lease => {
    const featID = String(field(lease, 'feature_id', 'FeatureID') || '');
    if (featID) leasesByFeature.set(featID, lease);
  });

  const attemptsByFeature = new Map();
  allAttempts.forEach(attempt => {
    const featID = String(field(attempt, 'feature_id', 'FeatureID') || '');
    if (featID) {
      if (!attemptsByFeature.has(featID)) attemptsByFeature.set(featID, []);
      attemptsByFeature.get(featID).push(attempt);
    }
  });

  const overlapsByFeature = new Map();
  allOverlaps.forEach(overlap => {
    const featID = String(field(overlap, 'feature_id', 'FeatureID') || '');
    if (featID) {
      if (!overlapsByFeature.has(featID)) overlapsByFeature.set(featID, []);
      overlapsByFeature.get(featID).push(overlap);
    }
  });

  const reconciliationsByFeature = new Map();
  allReconciliations.forEach(recon => {
    const featID = String(field(recon, 'feature_id', 'FeatureID') || '');
    if (featID) {
      if (!reconciliationsByFeature.has(featID)) reconciliationsByFeature.set(featID, []);
      reconciliationsByFeature.get(featID).push(recon);
    }
  });

  const normalized = new Map();

  rawFeatures.forEach((feature, index) => {
    const featureID = String(field(feature, 'id', 'ID', 'feature_id', 'FeatureID') || `feature-${index + 1}`);
    const featureStatus = String(field(feature, 'status', 'Status') || 'active').toLowerCase();
    const parentRef = String(field(feature, 'parent_ref', 'ParentRef') || 'Unavailable');
    const baseSHA = String(field(feature, 'base_sha', 'BaseSHA') || 'Unavailable');
    const expectedParentSHA = String(field(feature, 'expected_parent_sha', 'ExpectedParentSHA') || 'Unavailable');

    // Lease
    const leaseData = field(feature, 'lease', 'Lease') || leasesByFeature.get(featureID) || {};
    const leaseOwner = String(field(leaseData, 'owner', 'Owner', 'holder', 'Holder', 'lease_owner', 'LeaseOwner') || 'None');
    const leaseFenceRaw = field(leaseData, 'fence', 'Fence', 'lease_fence', 'LeaseFence');
    const leaseFence = leaseFenceRaw !== undefined && leaseFenceRaw !== null ? String(leaseFenceRaw) : '0';
    const leaseExpiresAt = field(leaseData, 'expires_at', 'ExpiresAt', 'lease_expires_at', 'LeaseExpiresAt');
    const leaseTTL = formatTTL(leaseExpiresAt, now);
    const leaseStatus = getLeaseStatus(leaseExpiresAt, now);

    // Attempts
    const rawAttempts = [
      ...asArray(feature, 'attempts', 'Attempts', 'integration_attempts', 'IntegrationAttempts'),
      ...(attemptsByFeature.get(featureID) || [])
    ];
    const attempts = rawAttempts.map((attempt, aIdx) => {
      const attemptID = String(field(attempt, 'id', 'ID', 'attempt_id', 'AttemptID') || `attempt-${aIdx + 1}`);
      const attemptStatus = String(field(attempt, 'status', 'Status') || 'recorded').toLowerCase();
      const candidateSHA = String(field(attempt, 'candidate_sha', 'CandidateSHA') || 'Unavailable');
      const failureReason = String(field(attempt, 'failure_reason', 'FailureReason', 'reason', 'Reason') || '');
      const fence = field(attempt, 'fence', 'Fence');
      const owner = field(attempt, 'owner', 'Owner');
      return {
        id: attemptID,
        status: attemptStatus,
        candidateSHA,
        failureReason,
        fence: fence !== undefined && fence !== null ? String(fence) : '',
        owner: owner ? String(owner) : ''
      };
    });

    // Overlap Evidence
    const rawOverlaps = [
      ...asArray(feature, 'overlap_evidence', 'OverlapEvidence', 'overlaps', 'Overlaps', 'overlap', 'Overlap'),
      ...(overlapsByFeature.get(featureID) || [])
    ];
    const overlapEvidence = rawOverlaps.map((overlap, oIdx) => {
      const overlapID = String(field(overlap, 'id', 'ID') || `overlap-${oIdx + 1}`);
      const version = String(field(overlap, 'version', 'Version') || '');
      const evidenceClass = String(field(overlap, 'evidence_class', 'EvidenceClass', 'class', 'Class') || 'informational').toLowerCase();
      const evidenceHash = String(field(overlap, 'evidence_hash', 'EvidenceHash', 'hash', 'Hash') || '');
      let evidenceJSON = field(overlap, 'evidence_json', 'EvidenceJSON', 'json', 'JSON', 'evidence', 'Evidence');
      if (typeof evidenceJSON === 'object' && evidenceJSON !== null) {
        evidenceJSON = JSON.stringify(evidenceJSON, null, 2);
      } else {
        evidenceJSON = String(evidenceJSON || '');
      }
      return {
        id: overlapID,
        version,
        evidenceClass,
        evidenceHash,
        evidenceJSON
      };
    });

    // Reconciliations
    const rawReconciliations = [
      ...asArray(feature, 'reconciliations', 'Reconciliations', 'reconciliation_requests', 'ReconciliationRequests'),
      ...(reconciliationsByFeature.get(featureID) || [])
    ];
    const reconciliations = rawReconciliations.map((recon, rIdx) => {
      const reconID = String(field(recon, 'id', 'ID') || `recon-${rIdx + 1}`);
      const reconStatus = String(field(recon, 'status', 'Status') || 'awaiting').toLowerCase();
      const direction = String(field(recon, 'direction', 'Direction') || '');
      const actor = String(field(recon, 'actor', 'Actor') || '');
      const reconExpiresAt = field(recon, 'expires_at', 'ExpiresAt');
      return {
        id: reconID,
        status: reconStatus,
        direction,
        actor,
        expiresAt: reconExpiresAt
      };
    });

    // Determine states
    const hasReconRequired = reconciliations.some(r => r.status === 'awaiting') ||
      overlapEvidence.some(o => o.evidenceClass === 'required') ||
      Boolean(field(feature, 'reconciliation_required', 'ReconciliationRequired'));

    const isPromoted = attempts.some(a => a.status === 'promoted') || featureStatus === 'promoted';
    const isBlocked = attempts.some(a => a.status === 'blocked') || featureStatus === 'blocked';
    const isExpired = leaseStatus === 'expired' || featureStatus === 'expired';

    let primaryStatus = featureStatus;
    if (isPromoted) primaryStatus = 'promoted';
    else if (isBlocked) primaryStatus = 'blocked';
    else if (hasReconRequired) primaryStatus = 'reconciliation-required';
    else if (isExpired) primaryStatus = 'expired';
    else if (featureStatus === 'active') primaryStatus = 'active';

    normalized.set(featureID, {
      key: featureID,
      id: featureID,
      status: primaryStatus,
      featureStatus,
      parentRef,
      baseSHA,
      expectedParentSHA,
      lease: {
        owner: leaseOwner,
        fence: leaseFence,
        expiresAt: leaseExpiresAt,
        ttl: leaseTTL,
        status: leaseStatus
      },
      attempts,
      overlapEvidence,
      reconciliations,
      reconciliationRequired: hasReconRequired,
      isPromoted,
      isBlocked,
      isExpired
    });
  });

  return Array.from(normalized.values());
}

function buildFeatureSwimlaneDOM(card, feature) {
  card.dataset.status = feature.status;
  card.dataset.featureStatus = feature.featureStatus;
  card.dataset.leaseStatus = feature.lease.status;
  if (feature.reconciliationRequired) {
    card.setAttribute('data-reconcile-required', 'true');
  } else {
    card.removeAttribute('data-reconcile-required');
  }

  const fragment = document.createDocumentFragment();

  // Header
  const header = document.createElement('div');
  header.className = 'swimlane-header';

  const titleGroup = document.createElement('div');
  titleGroup.className = 'swimlane-title-group';
  const featureID = document.createElement('span');
  featureID.className = 'feature-id';
  featureID.textContent = `Feature: ${feature.id}`;
  const statusBadge = document.createElement('span');
  statusBadge.className = 'badge badge-feature-status';
  statusBadge.textContent = feature.featureStatus;
  titleGroup.append(featureID, statusBadge);

  const badges = document.createElement('div');
  badges.className = 'feature-badges';

  if (feature.reconciliationRequired) {
    const reconBadge = document.createElement('span');
    reconBadge.className = 'badge badge-reconcile';
    reconBadge.setAttribute('data-reconcile-badge', 'required');
    reconBadge.textContent = 'Reconciliation required';
    badges.appendChild(reconBadge);
  }

  if (feature.lease && feature.lease.owner && feature.lease.owner !== 'None') {
    const leaseBadge = document.createElement('span');
    leaseBadge.className = 'badge badge-lease';
    leaseBadge.setAttribute('data-lease-status', feature.lease.status);
    leaseBadge.textContent = `Lease: ${feature.lease.status} (${feature.lease.ttl})`;
    badges.appendChild(leaseBadge);
  }

  if (feature.isPromoted) {
    const promotedBadge = document.createElement('span');
    promotedBadge.className = 'badge badge-promoted';
    promotedBadge.setAttribute('data-attempt-status', 'promoted');
    promotedBadge.textContent = 'Promoted';
    badges.appendChild(promotedBadge);
  }

  if (feature.isBlocked) {
    const blockedBadge = document.createElement('span');
    blockedBadge.className = 'badge badge-blocked';
    blockedBadge.setAttribute('data-attempt-status', 'blocked');
    blockedBadge.textContent = 'Blocked';
    badges.appendChild(blockedBadge);
  }

  if (feature.isExpired) {
    const expiredBadge = document.createElement('span');
    expiredBadge.className = 'badge badge-expired';
    expiredBadge.setAttribute('data-status', 'expired');
    expiredBadge.textContent = 'Expired';
    badges.appendChild(expiredBadge);
  }

  header.append(titleGroup, badges);
  fragment.appendChild(header);

  // Grid of sections
  const grid = document.createElement('div');
  grid.className = 'swimlane-grid';

  // 1. Refs Section
  const refsSection = document.createElement('section');
  refsSection.className = 'swimlane-section refs-section';
  refsSection.setAttribute('aria-label', 'Parent and base references');
  refsSection.appendChild(createTextElement('h4', 'swimlane-section-title', 'Parent / Base refs'));

  const addRefItem = (label, val) => {
    const item = document.createElement('div');
    item.className = 'ref-item';
    item.appendChild(createTextElement('span', 'ref-label', label));
    item.appendChild(createTextElement('span', 'ref-value', val));
    refsSection.appendChild(item);
  };
  addRefItem('Parent ref', feature.parentRef);
  addRefItem('Base SHA', feature.baseSHA);
  addRefItem('Expected parent SHA', feature.expectedParentSHA);
  grid.appendChild(refsSection);

  // 2. Lease & TTL Section
  const leaseSection = document.createElement('section');
  leaseSection.className = 'swimlane-section lease-section';
  leaseSection.setAttribute('aria-label', 'Lease holder fence and TTL');
  leaseSection.appendChild(createTextElement('h4', 'swimlane-section-title', 'Lease & TTL'));

  const addLeaseItem = (label, val) => {
    const item = document.createElement('div');
    item.className = 'lease-item';
    item.appendChild(createTextElement('span', 'lease-label', label));
    item.appendChild(createTextElement('span', 'lease-value', val));
    leaseSection.appendChild(item);
  };
  addLeaseItem('Lease holder', feature.lease.owner);
  addLeaseItem('Lease fence', feature.lease.fence);
  addLeaseItem('Live TTL', feature.lease.ttl);
  if (feature.lease.expiresAt) {
    addLeaseItem('Expires at', String(feature.lease.expiresAt));
  }
  grid.appendChild(leaseSection);

  // 3. Attempts Section
  const attemptsSection = document.createElement('section');
  attemptsSection.className = 'swimlane-section attempts-section';
  attemptsSection.setAttribute('aria-label', 'Integration attempts');
  attemptsSection.appendChild(createTextElement('h4', 'swimlane-section-title', 'Integration attempts'));

  if (feature.attempts.length === 0) {
    attemptsSection.appendChild(createTextElement('div', 'empty-note', 'No integration attempts recorded.'));
  } else {
    feature.attempts.forEach(attempt => {
      const attemptCard = document.createElement('div');
      attemptCard.className = 'attempt-card';
      attemptCard.setAttribute('data-attempt-id', attempt.id);
      attemptCard.setAttribute('data-attempt-status', attempt.status);
      attemptCard.dataset.status = attempt.status;

      const attemptHeader = document.createElement('div');
      attemptHeader.className = 'attempt-header';
      attemptHeader.appendChild(createTextElement('strong', 'attempt-id', attempt.id));
      attemptHeader.appendChild(createTextElement('span', 'attempt-status', attempt.status));
      attemptCard.appendChild(attemptHeader);

      const candItem = document.createElement('div');
      candItem.className = 'ref-item';
      candItem.appendChild(createTextElement('span', 'attempt-label', 'Candidate SHA'));
      candItem.appendChild(createTextElement('span', 'attempt-value', attempt.candidateSHA));
      attemptCard.appendChild(candItem);

      if (attempt.failureReason) {
        const failItem = document.createElement('div');
        failItem.className = 'attempt-failure';
        failItem.appendChild(createTextElement('span', 'attempt-label', 'Failure reason'));
        const pre = document.createElement('pre');
        pre.className = 'failure-reason';
        pre.textContent = attempt.failureReason;
        failItem.appendChild(pre);
        attemptCard.appendChild(failItem);
      }
      attemptsSection.appendChild(attemptCard);
    });
  }
  grid.appendChild(attemptsSection);

  // 4. Overlap Evidence Section
  const overlapSection = document.createElement('section');
  overlapSection.className = 'swimlane-section overlap-section';
  overlapSection.setAttribute('aria-label', 'Overlap evidence');
  overlapSection.appendChild(createTextElement('h4', 'swimlane-section-title', 'Overlap evidence'));

  if (feature.overlapEvidence.length === 0) {
    overlapSection.appendChild(createTextElement('div', 'empty-note', 'No overlap evidence recorded.'));
  } else {
    feature.overlapEvidence.forEach(overlap => {
      const overlapCard = document.createElement('div');
      overlapCard.className = 'overlap-card';
      overlapCard.setAttribute('data-evidence-class', overlap.evidenceClass);

      const overlapHeader = document.createElement('div');
      overlapHeader.className = 'overlap-header';
      const classBadge = document.createElement('span');
      classBadge.className = 'evidence-class-badge';
      classBadge.setAttribute('data-class', overlap.evidenceClass);
      classBadge.textContent = `Class: ${overlap.evidenceClass}`;
      const hashSpan = document.createElement('span');
      hashSpan.className = 'ref-value';
      hashSpan.textContent = `Hash: ${overlap.evidenceHash || '-'}`;
      overlapHeader.append(classBadge, hashSpan);
      overlapCard.appendChild(overlapHeader);

      if (overlap.version) {
        const verItem = document.createElement('div');
        verItem.className = 'ref-item';
        verItem.appendChild(createTextElement('span', 'ref-label', 'Version'));
        verItem.appendChild(createTextElement('span', 'ref-value', overlap.version));
        overlapCard.appendChild(verItem);
      }

      if (overlap.evidenceJSON) {
        const pre = document.createElement('pre');
        pre.className = 'overlap-evidence-json';
        pre.textContent = overlap.evidenceJSON;
        overlapCard.appendChild(pre);
      }
      overlapSection.appendChild(overlapCard);
    });
  }
  grid.appendChild(overlapSection);

  // 5. Reconciliation Section
  const reconSection = document.createElement('section');
  reconSection.className = 'swimlane-section reconcile-section';
  reconSection.setAttribute('aria-label', 'Reconciliation requests');
  reconSection.appendChild(createTextElement('h4', 'swimlane-section-title', 'Reconciliation'));

  if (feature.reconciliations.length === 0) {
    reconSection.appendChild(createTextElement('div', 'empty-note', 'No reconciliation requests.'));
  } else {
    feature.reconciliations.forEach(recon => {
      const reconCard = document.createElement('div');
      reconCard.className = 'reconcile-card';
      reconCard.setAttribute('data-reconcile-status', recon.status);
      reconCard.dataset.status = recon.status;

      const reconHeader = document.createElement('div');
      reconHeader.className = 'reconcile-header';
      reconHeader.appendChild(createTextElement('strong', 'recon-id', recon.id));
      reconHeader.appendChild(createTextElement('span', 'recon-status', `Status: ${recon.status}`));
      reconCard.appendChild(reconHeader);

      if (recon.direction) {
        const dirItem = document.createElement('div');
        dirItem.className = 'recon-field';
        dirItem.appendChild(createTextElement('span', 'recon-label', 'Direction'));
        dirItem.appendChild(createTextElement('span', 'recon-value', recon.direction));
        reconCard.appendChild(dirItem);
      }
      if (recon.actor) {
        const actorItem = document.createElement('div');
        actorItem.className = 'recon-field';
        actorItem.appendChild(createTextElement('span', 'recon-label', 'Actor'));
        actorItem.appendChild(createTextElement('span', 'recon-value', recon.actor));
        reconCard.appendChild(actorItem);
      }
      reconSection.appendChild(reconCard);
    });
  }
  grid.appendChild(reconSection);

  fragment.appendChild(grid);
  card.replaceChildren(fragment);
}

function createFeatureSwimlaneCard(feature) {
  const card = document.createElement('article');
  card.className = 'feature-swimlane';
  card.setAttribute('data-feature-key', feature.key);
  buildFeatureSwimlaneDOM(card, feature);
  return card;
}

function updateFeatureSwimlaneCard(card, feature) {
  card.setAttribute('data-feature-key', feature.key);
  buildFeatureSwimlaneDOM(card, feature);
}

function patchFeatureSwimlanes(container, features) {
  const currentCards = new Map();
  container.querySelectorAll('[data-feature-key]').forEach(card => {
    currentCards.set(card.getAttribute('data-feature-key'), card);
  });

  const activeKeys = new Set();
  features.forEach(feature => {
    activeKeys.add(feature.key);
    const existing = currentCards.get(feature.key);
    if (existing) {
      updateFeatureSwimlaneCard(existing, feature);
    } else {
      container.appendChild(createFeatureSwimlaneCard(feature));
    }
  });

  currentCards.forEach((card, key) => {
    if (!activeKeys.has(key)) card.remove();
  });

  let emptyState = container.querySelector('[data-features-empty]');
  if (features.length === 0 && !emptyState) {
    emptyState = document.createElement('div');
    emptyState.className = 'features-empty empty-state';
    emptyState.setAttribute('data-features-empty', '');
    emptyState.textContent = 'No features reported.';
    container.appendChild(emptyState);
  } else if (features.length > 0 && emptyState) {
    emptyState.remove();
  }
}

function renderFeatureSwimlanes(container, features) {
  if (!container) return;
  patchFeatureSwimlanes(container, Array.isArray(features) ? features : []);
}

const TIMELINE_WINDOW_SIZE = 50;
const TIMELINE_STYLES = `
  .timeline-view { display: grid; gap: 1.25rem; margin-top: 1.5rem; }
  .timeline-controls { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 0.75rem; padding: 0.75rem; border: 1px solid var(--line); background: var(--surface); }
  .timeline-kind-filters { display: flex; gap: 0.4rem; flex-wrap: wrap; }
  .timeline-filter-btn { padding: 0.35rem 0.65rem; border: 1px solid var(--line-strong); background: var(--surface-raised); color: var(--muted); cursor: pointer; font-family: var(--font-mono); font-size: 0.72rem; text-transform: uppercase; letter-spacing: 0.05em; }
  .timeline-filter-btn[aria-pressed="true"] { border-color: var(--signal); color: var(--signal); background: var(--surface-deep); font-weight: 700; }
  .timeline-search-box { display: flex; align-items: center; gap: 0.5rem; }
  .timeline-search-input { padding: 0.35rem 0.65rem; border: 1px solid var(--line-strong); background: var(--surface-deep); color: var(--ink); font-family: var(--font-mono); font-size: 0.76rem; }
  .timeline-search-input:focus { outline: 2px solid var(--signal); }
  .timeline-window-bar { display: flex; justify-content: space-between; align-items: center; padding: 0.5rem 0.75rem; border: 1px solid var(--line); background: var(--surface-deep); font-family: var(--font-mono); font-size: 0.72rem; color: var(--muted); }
  .timeline-count-badge { color: var(--ink); font-weight: 600; }
  .timeline-load-more { padding: 0.3rem 0.65rem; border: 1px solid var(--line-strong); background: var(--surface-raised); color: var(--signal); cursor: pointer; font-family: var(--font-mono); font-size: 0.72rem; }
  .timeline-load-more:disabled { opacity: 0.5; cursor: default; }
  .timeline-feed { display: grid; gap: 0.75rem; }
  .timeline-card { min-width: 0; padding: 0.85rem; border: 1px solid var(--line); background: var(--surface-raised); box-shadow: 4px 4px 0 rgba(0, 0, 0, 0.2); }
  .timeline-card[data-kind="event"] { border-left: 5px solid var(--signal); }
  .timeline-card[data-kind="integration"] { border-left: 5px solid var(--warning); }
  .timeline-card[data-kind="progress"] { border-left: 5px solid var(--live); }
  .timeline-card-header { display: flex; justify-content: space-between; align-items: center; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 0.5rem; font-family: var(--font-mono); font-size: 0.76rem; }
  .timeline-card-title { display: flex; align-items: center; gap: 0.5rem; }
  .timeline-badge { padding: 0.2rem 0.45rem; border: 1px solid var(--line-strong); font-size: 0.65rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; background: var(--surface); }
  .timeline-badge.badge-event { color: var(--signal); border-color: var(--signal); }
  .timeline-badge.badge-integration { color: var(--warning); border-color: var(--warning); }
  .timeline-badge.badge-progress { color: var(--live); border-color: var(--live); }
  .timeline-type { font-weight: 700; color: var(--ink); }
  .timeline-time { color: var(--muted); font-size: 0.72rem; }
  .timeline-meta { display: flex; gap: 0.75rem; flex-wrap: wrap; margin-bottom: 0.4rem; font-family: var(--font-mono); font-size: 0.72rem; color: var(--muted); }
  .timeline-meta-item strong { color: var(--ink); }
  .timeline-detail-text { margin: 0.4rem 0 0; padding: 0.5rem; border: 1px solid var(--line); background: var(--surface-deep); color: #d7dcd7; font-family: var(--font-mono); font-size: 0.72rem; line-height: 1.45; white-space: pre-wrap; overflow-wrap: anywhere; }
  .timeline-empty { padding: 2.4rem; border: 1px dashed var(--line-strong); background: var(--surface); color: var(--muted); }
`;

function ensureTimelineContainer() {
  let container = document.getElementById('timeline-container');
  if (container) return container;

  const activityView = document.getElementById('activity-view');
  if (!activityView) return null;

  if (!document.getElementById('timeline-styles')) {
    const style = document.createElement('style');
    style.id = 'timeline-styles';
    style.textContent = TIMELINE_STYLES;
    document.head.appendChild(style);
  }

  container = document.createElement('section');
  container.id = 'timeline-container';
  container.className = 'timeline-view';
  container.setAttribute('aria-labelledby', 'timeline-title');
  container.appendChild(createTextElement('h2', 'sdd-change-title', 'Execution timeline'));
  container.firstChild.id = 'timeline-title';
  activityView.appendChild(container);
  return container;
}

function parseTimelineTimestamp(at) {
  if (!at) return 0;
  const parsed = Date.parse(at);
  return Number.isFinite(parsed) ? parsed : 0;
}

function sortTimelineItems(items) {
  const kindPriority = { event: 1, integration: 2, progress: 3 };
  return [...items].sort((a, b) => {
    const timeA = parseTimelineTimestamp(a.at);
    const timeB = parseTimelineTimestamp(b.at);
    if (timeA !== timeB) return timeA - timeB;
    if (a.at !== b.at) return String(a.at || '').localeCompare(String(b.at || ''));
    const prioA = kindPriority[a.kind] || 99;
    const prioB = kindPriority[b.kind] || 99;
    if (prioA !== prioB) return prioA - prioB;
    const seqOrIdA = Number(a.seq || a.id || 0);
    const seqOrIdB = Number(b.seq || b.id || 0);
    if (seqOrIdA !== seqOrIdB) return seqOrIdA - seqOrIdB;
    return String(a.key).localeCompare(String(b.key));
  });
}

function filterAfterCursor(items, cursor) {
  if (!cursor) return items;
  const index = items.findIndex(item => item.cursor === cursor || item.key === cursor);
  if (index === -1) return items;
  return items.slice(index + 1);
}

function filterTimelineItems(items, filters = {}) {
  const kind = (filters.kind || 'all').toLowerCase();
  const search = (filters.search || '').trim().toLowerCase();
  const runID = (filters.runID || '').trim().toLowerCase();
  const laneID = (filters.laneID || '').trim().toLowerCase();
  const featureID = (filters.featureID || '').trim().toLowerCase();
  const type = (filters.type || '').trim().toLowerCase();

  return items.filter(item => {
    if (kind !== 'all' && item.kind !== kind) return false;
    if (runID && item.runID.toLowerCase() !== runID) return false;
    if (laneID && item.laneID.toLowerCase() !== laneID) return false;
    if (featureID && item.featureID.toLowerCase() !== featureID) return false;
    if (type && item.type.toLowerCase() !== type) return false;
    if (search) {
      const haystack = [
        item.kind,
        item.kindLabel,
        item.type,
        item.detail,
        item.runID,
        item.laneID,
        item.featureID,
        item.attemptID,
        item.cursor
      ].join(' ').toLowerCase();
      if (!haystack.includes(search)) return false;
    }
    return true;
  });
}

function virtualizeTimeline(items, { offset = 0, limit = TIMELINE_WINDOW_SIZE } = {}) {
  const total = items.length;
  const clampedLimit = Math.max(1, limit);
  const clampedOffset = Math.max(0, Math.min(offset, Math.max(0, total - 1)));
  const visible = items.slice(clampedOffset, clampedOffset + clampedLimit);
  const hasMore = clampedOffset + clampedLimit < total;
  const nextCursor = hasMore && visible.length > 0 ? visible[visible.length - 1].cursor : null;

  return {
    items: visible,
    total,
    offset: clampedOffset,
    limit: clampedLimit,
    visibleCount: visible.length,
    hasMore,
    nextCursor
  };
}

function normalizeTimeline(state) {
  const normalized = new Map();

  // 1. Run Events
  const rawEvents = [
    ...asArray(state, 'events', 'Events', 'run_events', 'RunEvents')
  ];
  const runs = asArray(state, 'runs', 'Runs');
  runs.forEach(run => {
    const runEvents = asArray(run, 'events', 'Events', 'run_events', 'RunEvents');
    runEvents.forEach(item => {
      if (typeof item === 'object' && item !== null) {
        rawEvents.push({ ...item, run_id: field(item, 'run_id', 'RunID') || field(run, 'run_id', 'RunID') });
      }
    });
  });

  rawEvents.forEach((item, index) => {
    if (!item || typeof item !== 'object') return;
    const id = field(item, 'id', 'ID') !== undefined && field(item, 'id', 'ID') !== null ? String(field(item, 'id', 'ID')) : String(index + 1);
    const runID = String(field(item, 'run_id', 'RunID') || '');
    const laneID = String(field(item, 'lane_id', 'LaneID') || '');
    const type = String(field(item, 'type', 'Type') || 'event');
    const detail = String(field(item, 'detail', 'Detail', 'message', 'Message') || '');
    const at = String(field(item, 'at', 'At', 'timestamp', 'Timestamp') || '');
    const key = `event-${runID}-${laneID}-${id}`;
    const cursor = `event:${runID}:${id}`;

    normalized.set(key, {
      key,
      kind: 'event',
      kindLabel: 'Run event',
      id,
      runID,
      laneID,
      featureID: '',
      attemptID: '',
      type,
      detail,
      at,
      timestamp: parseTimelineTimestamp(at),
      cursor
    });
  });

  // 2. Integration Events
  const rawIntegrationEvents = [
    ...asArray(state, 'integration_events', 'IntegrationEvents', 'audit_events', 'AuditEvents')
  ];
  const features = asArray(state, 'features', 'Features');
  features.forEach(feat => {
    const featAudit = asArray(feat, 'audit', 'Audit', 'events', 'Events', 'integration_events', 'IntegrationEvents');
    featAudit.forEach(item => {
      if (typeof item === 'object' && item !== null) {
        rawIntegrationEvents.push({ ...item, feature_id: field(item, 'feature_id', 'FeatureID') || field(feat, 'id', 'ID') });
      }
    });
  });
  const reconciliations = asArray(state, 'reconciliations', 'Reconciliations', 'reconciliation_requests', 'ReconciliationRequests');
  reconciliations.forEach(recon => {
    const reconAudit = asArray(recon, 'audit', 'Audit', 'events', 'Events');
    reconAudit.forEach(item => {
      if (typeof item === 'object' && item !== null) {
        rawIntegrationEvents.push({ ...item, feature_id: field(item, 'feature_id', 'FeatureID') || field(recon, 'feature_id', 'FeatureID') });
      }
    });
  });

  rawIntegrationEvents.forEach((item, index) => {
    if (!item || typeof item !== 'object') return;
    const id = field(item, 'id', 'ID') !== undefined && field(item, 'id', 'ID') !== null ? String(field(item, 'id', 'ID')) : String(index + 1);
    const featureID = String(field(item, 'feature_id', 'FeatureID') || '');
    const attemptID = String(field(item, 'attempt_id', 'AttemptID') || '');
    const type = String(field(item, 'type', 'Type') || 'integration');
    const detail = String(field(item, 'detail', 'Detail', 'message', 'Message') || '');
    const at = String(field(item, 'at', 'At', 'timestamp', 'Timestamp') || '');
    const key = `integration-${featureID}-${attemptID}-${id}`;
    const cursor = `integration:${featureID}:${id}`;

    normalized.set(key, {
      key,
      kind: 'integration',
      kindLabel: 'Integration audit',
      id,
      runID: '',
      laneID: '',
      featureID,
      attemptID,
      type,
      detail,
      at,
      timestamp: parseTimelineTimestamp(at),
      cursor
    });
  });

  // 3. Lane Progress
  const rawProgress = [
    ...asArray(state, 'lane_progress', 'LaneProgress', 'progress', 'Progress')
  ];
  runs.forEach(run => {
    const runProg = asArray(run, 'lane_progress', 'LaneProgress', 'progress', 'Progress');
    runProg.forEach(item => {
      if (typeof item === 'object' && item !== null) {
        rawProgress.push({ ...item, run_id: field(item, 'run_id', 'RunID') || field(run, 'run_id', 'RunID') });
      }
    });
  });
  const lanes = asArray(state, 'lanes', 'Lanes');
  lanes.forEach(lane => {
    const laneProg = asArray(lane, 'lane_progress', 'LaneProgress', 'progress', 'Progress');
    laneProg.forEach(item => {
      if (typeof item === 'object' && item !== null) {
        rawProgress.push({
          ...item,
          run_id: field(item, 'run_id', 'RunID') || field(lane, 'run_id', 'RunID'),
          lane_id: field(item, 'lane_id', 'LaneID') || field(lane, 'lane_id', 'LaneID', 'id', 'ID')
        });
      }
    });
  });

  rawProgress.forEach((item, index) => {
    if (!item || typeof item !== 'object') return;
    const seq = field(item, 'seq', 'Seq') !== undefined && field(item, 'seq', 'Seq') !== null ? String(field(item, 'seq', 'Seq')) : String(index + 1);
    const runID = String(field(item, 'run_id', 'RunID') || '');
    const laneID = String(field(item, 'lane_id', 'LaneID') || '');
    const message = String(field(item, 'message', 'Message', 'activity', 'Activity') || '');
    const at = String(field(item, 'at', 'At', 'timestamp', 'Timestamp') || '');
    const key = `progress-${runID}-${laneID}-${seq}`;
    const cursor = `progress:${runID}:${laneID}:${seq}`;

    normalized.set(key, {
      key,
      kind: 'progress',
      kindLabel: 'Lane progress',
      seq,
      runID,
      laneID,
      featureID: '',
      attemptID: '',
      type: 'progress',
      detail: message,
      at,
      timestamp: parseTimelineTimestamp(at),
      cursor
    });
  });

  return Array.from(normalized.values());
}

function createTimelineCard(item) {
  const card = document.createElement('article');
  card.className = 'timeline-card';
  card.setAttribute('data-timeline-key', item.key);
  card.setAttribute('data-kind', item.kind);
  card.dataset.kind = item.kind;
  card.setAttribute('data-event-type', item.type);
  card.dataset.eventType = item.type;
  if (item.runID) card.setAttribute('data-run-id', item.runID);
  if (item.laneID) card.setAttribute('data-lane-id', item.laneID);
  if (item.featureID) card.setAttribute('data-feature-id', item.featureID);
  if (item.cursor) card.setAttribute('data-cursor', item.cursor);

  const header = document.createElement('div');
  header.className = 'timeline-card-header';

  const titleGroup = document.createElement('div');
  titleGroup.className = 'timeline-card-title';

  const badge = document.createElement('span');
  badge.className = `timeline-badge badge-${item.kind}`;
  badge.textContent = item.kindLabel;

  const typeName = document.createElement('strong');
  typeName.className = 'timeline-type';
  typeName.textContent = item.type;
  titleGroup.append(badge, typeName);

  const timeNode = document.createElement('time');
  timeNode.className = 'timeline-time';
  timeNode.textContent = item.at || 'Unavailable';
  if (item.at) timeNode.setAttribute('datetime', item.at);
  header.append(titleGroup, timeNode);

  const meta = document.createElement('div');
  meta.className = 'timeline-meta';
  const metaParts = [];
  if (item.runID) metaParts.push(`Run: ${item.runID}`);
  if (item.laneID) metaParts.push(`Lane: ${item.laneID}`);
  if (item.featureID) metaParts.push(`Feature: ${item.featureID}`);
  if (item.attemptID) metaParts.push(`Attempt: ${item.attemptID}`);
  if (item.seq !== undefined && item.seq !== null && item.seq !== '') metaParts.push(`Seq: ${item.seq}`);
  if (item.id !== undefined && item.id !== null && item.id !== '') metaParts.push(`ID: ${item.id}`);

  metaParts.forEach(text => {
    const span = document.createElement('span');
    span.className = 'timeline-meta-item';
    span.textContent = text;
    meta.appendChild(span);
  });

  const detailBox = document.createElement('div');
  detailBox.className = 'timeline-detail';
  const pre = document.createElement('pre');
  pre.className = 'timeline-detail-text';
  pre.textContent = item.detail || '(no detail)';
  detailBox.appendChild(pre);

  card.append(header, meta, detailBox);
  card._timelineParts = { badge, typeName, timeNode, meta, pre };
  return card;
}

function updateTimelineCard(card, item) {
  card.setAttribute('data-timeline-key', item.key);
  card.setAttribute('data-kind', item.kind);
  card.dataset.kind = item.kind;
  card.setAttribute('data-event-type', item.type);
  card.dataset.eventType = item.type;
  if (item.runID) card.setAttribute('data-run-id', item.runID);
  if (item.laneID) card.setAttribute('data-lane-id', item.laneID);
  if (item.featureID) card.setAttribute('data-feature-id', item.featureID);
  if (item.cursor) card.setAttribute('data-cursor', item.cursor);

  const parts = card._timelineParts;
  if (!parts) return;
  parts.badge.className = `timeline-badge badge-${item.kind}`;
  parts.badge.textContent = item.kindLabel;
  parts.typeName.textContent = item.type;
  parts.timeNode.textContent = item.at || 'Unavailable';
  if (item.at) parts.timeNode.setAttribute('datetime', item.at);

  const metaParts = [];
  if (item.runID) metaParts.push(`Run: ${item.runID}`);
  if (item.laneID) metaParts.push(`Lane: ${item.laneID}`);
  if (item.featureID) metaParts.push(`Feature: ${item.featureID}`);
  if (item.attemptID) metaParts.push(`Attempt: ${item.attemptID}`);
  if (item.seq !== undefined && item.seq !== null && item.seq !== '') metaParts.push(`Seq: ${item.seq}`);
  if (item.id !== undefined && item.id !== null && item.id !== '') metaParts.push(`ID: ${item.id}`);

  parts.meta.replaceChildren(
    ...metaParts.map(text => {
      const span = document.createElement('span');
      span.className = 'timeline-meta-item';
      span.textContent = text;
      return span;
    })
  );

  parts.pre.textContent = item.detail || '(no detail)';
}

function patchTimelineItems(feedContainer, visibleItems, totalFiltered, totalRaw) {
  const currentCards = new Map();
  feedContainer.querySelectorAll('[data-timeline-key]').forEach(card => {
    currentCards.set(card.getAttribute('data-timeline-key'), card);
  });

  if (totalFiltered === 0) {
    currentCards.forEach(card => card.remove());
    let emptyState = feedContainer.querySelector('[data-timeline-empty]');
    if (!emptyState) {
      emptyState = document.createElement('div');
      emptyState.className = 'timeline-empty empty-state';
      emptyState.setAttribute('data-timeline-empty', '');
      feedContainer.appendChild(emptyState);
    }
    emptyState.textContent = totalRaw === 0
      ? 'No timeline events reported.'
      : 'No timeline events match the selected filters.';
    return;
  }

  let emptyState = feedContainer.querySelector('[data-timeline-empty]');
  if (emptyState) emptyState.remove();

  const activeKeys = new Set();
  visibleItems.forEach(item => {
    activeKeys.add(item.key);
    const existing = currentCards.get(item.key);
    if (existing) {
      updateTimelineCard(existing, item);
    } else {
      feedContainer.appendChild(createTimelineCard(item));
    }
  });

  currentCards.forEach((card, key) => {
    if (!activeKeys.has(key)) card.remove();
  });
}

function renderTimeline(container, rawItems) {
  if (!container) return;

  if (!container._timelineState) {
    container._timelineState = {
      filters: { kind: 'all', search: '', runID: '', laneID: '', featureID: '', type: '' },
      windowLimit: TIMELINE_WINDOW_SIZE,
      afterCursor: null
    };
  }
  container._timelineRawItems = Array.isArray(rawItems) ? rawItems : [];

  let controls = container.querySelector('.timeline-controls');
  if (!controls) {
    controls = document.createElement('div');
    controls.className = 'timeline-controls';
    controls.setAttribute('role', 'toolbar');
    controls.setAttribute('aria-label', 'Timeline filters');

    const kindGroup = document.createElement('div');
    kindGroup.className = 'timeline-kind-filters';
    ['all', 'event', 'integration', 'progress'].forEach(k => {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'timeline-filter-btn';
      btn.setAttribute('data-filter-kind', k);
      btn.setAttribute('aria-pressed', String(k === container._timelineState.filters.kind));
      btn.textContent = k === 'all' ? 'All' : k === 'event' ? 'Events' : k === 'integration' ? 'Integration' : 'Progress';
      btn.addEventListener('click', () => {
        container._timelineState.filters.kind = k;
        container.querySelectorAll('[data-filter-kind]').forEach(b => {
          b.setAttribute('aria-pressed', String(b.getAttribute('data-filter-kind') === k));
        });
        renderTimeline(container, container._timelineRawItems);
      });
      kindGroup.appendChild(btn);
    });
    controls.appendChild(kindGroup);

    const searchBox = document.createElement('div');
    searchBox.className = 'timeline-search-box';
    const searchInput = document.createElement('input');
    searchInput.type = 'search';
    searchInput.className = 'timeline-search-input';
    searchInput.placeholder = 'Filter timeline…';
    searchInput.setAttribute('aria-label', 'Filter timeline events');
    searchInput.value = container._timelineState.filters.search;
    searchInput.addEventListener('input', e => {
      container._timelineState.filters.search = e.target.value;
      renderTimeline(container, container._timelineRawItems);
    });
    searchBox.appendChild(searchInput);
    controls.appendChild(searchBox);

    container.appendChild(controls);
  } else {
    // Update button states
    controls.querySelectorAll('[data-filter-kind]').forEach(btn => {
      btn.setAttribute('aria-pressed', String(btn.getAttribute('data-filter-kind') === container._timelineState.filters.kind));
    });
  }

  let windowBar = container.querySelector('.timeline-window-bar');
  if (!windowBar) {
    windowBar = document.createElement('div');
    windowBar.className = 'timeline-window-bar';
    windowBar.setAttribute('data-timeline-window', '');

    const countBadge = document.createElement('span');
    countBadge.className = 'timeline-count-badge';
    countBadge.setAttribute('data-timeline-count', '');

    const loadMoreBtn = document.createElement('button');
    loadMoreBtn.type = 'button';
    loadMoreBtn.className = 'timeline-load-more';
    loadMoreBtn.setAttribute('data-timeline-more', '');
    loadMoreBtn.textContent = 'Load more';
    loadMoreBtn.addEventListener('click', () => {
      container._timelineState.windowLimit += TIMELINE_WINDOW_SIZE;
      renderTimeline(container, container._timelineRawItems);
    });

    windowBar.append(countBadge, loadMoreBtn);
    container.appendChild(windowBar);
  }

  let feed = container.querySelector('.timeline-feed');
  if (!feed) {
    feed = document.createElement('div');
    feed.className = 'timeline-feed';
    feed.setAttribute('data-timeline-feed', '');
    feed.setAttribute('aria-live', 'polite');
    container.appendChild(feed);
  }

  // Pipeline: sort -> filter -> virtualize/window
  const sorted = sortTimelineItems(container._timelineRawItems);
  const filtered = filterTimelineItems(sorted, container._timelineState.filters);
  const windowed = virtualizeTimeline(filtered, { limit: container._timelineState.windowLimit });

  // Update window bar
  const countBadge = windowBar.querySelector('[data-timeline-count]');
  if (countBadge) {
    countBadge.textContent = `Showing ${windowed.visibleCount} of ${windowed.total} events (total feed: ${container._timelineRawItems.length})`;
  }
  const loadMoreBtn = windowBar.querySelector('[data-timeline-more]');
  if (loadMoreBtn) {
    loadMoreBtn.hidden = !windowed.hasMore;
    loadMoreBtn.disabled = !windowed.hasMore;
  }

  // Patch feed items
  patchTimelineItems(feed, windowed.items, windowed.total, container._timelineRawItems.length);
}

function renderState(state) {
  const approver = document.getElementById('approver-name');
  const rate = document.getElementById('approver-rate');
  const command = document.getElementById('opencode-cmd');
  const pendingCount = document.getElementById('pending-approvals-count');
  const approvalsContainer = document.getElementById('approvals-container');
  const fleetContainer = document.querySelector('[data-view-outlet="activity"] .activity-card');

  if (approver) approver.textContent = state.approver || '-';
  if (rate) rate.textContent = `${((state.approver_rate || 0) * 100).toFixed(1)}%`;
  if (command && state.opencode_command) command.textContent = state.opencode_command;
  if (approvalsContainer) {
    const count = patchApprovalCards(approvalsContainer, state.approvals || []);
    if (pendingCount) pendingCount.textContent = String(count);
  }
  syncServerClock(state);
  if (fleetContainer) patchFleetCards(fleetContainer, normalizeFleetState(state, serverNow()));
  renderApplyDAG(state.apply_dag);

  const sddFlows = Array.isArray(state.sdd_flows) ? state.sdd_flows : [];
  renderSDDFlows(ensureSDDFlowsContainer(), sddFlows);

  const features = normalizeFeatureSwimlanes(state, serverNow());
  renderFeatureSwimlanes(ensureFeatureSwimlanesContainer(), features);

  const timeline = normalizeTimeline(state);
  renderTimeline(ensureTimelineContainer(), timeline);
}

function createTextElement(tagName, className, text) {
  const element = document.createElement(tagName);
  if (className) element.className = className;
  element.textContent = text;
  return element;
}

function ensureSDDFlowsContainer() {
  let container = document.getElementById('sdd-flows-container');
  if (container) return container;

  const activityView = document.getElementById('activity-view');
  if (!activityView) return null;

  if (!document.getElementById('sdd-flow-styles')) {
    const style = document.createElement('style');
    style.id = 'sdd-flow-styles';
    style.textContent = SDD_FLOW_STYLES;
    document.head.appendChild(style);
  }

  container = document.createElement('section');
  container.id = 'sdd-flows-container';
  container.className = 'sdd-flows';
  container.setAttribute('aria-labelledby', 'sdd-flows-title');
  container.appendChild(createTextElement('h2', 'sdd-change-title', 'SDD flows'));
  container.firstChild.id = 'sdd-flows-title';
  activityView.appendChild(container);
  return container;
}

function normalizeSDDFlow(flow) {
  const normalized = {};
  SDD_FLOW_FIELDS.forEach(field => {
    const supplied = flow && Object.prototype.hasOwnProperty.call(flow, field);
    normalized[field] = supplied ? flow[field] : null;
  });
  normalized.sdd_phase = SDD_PHASE_ALIASES[normalized.sdd_phase] || normalized.sdd_phase;
  normalized.lane_ids = Array.isArray(normalized.lane_ids) ? normalized.lane_ids : [];
  return normalized;
}

function groupSDDFlowsByChange(flows) {
  const changes = new Map();
  flows.map(normalizeSDDFlow).forEach(flow => {
    const change = typeof flow.change === 'string' && flow.change ? flow.change : 'Unnamed change';
    if (!changes.has(change)) changes.set(change, new Map());
    const phases = changes.get(change);
    if (!phases.has(flow.sdd_phase)) phases.set(flow.sdd_phase, []);
    phases.get(flow.sdd_phase).push(flow);
  });
  return changes;
}

function sddFlowRole(flow, planning) {
  const group = typeof flow.fanout_group === 'string' ? flow.fanout_group.toLowerCase() : '';
  const laneNames = flow.lane_ids.join(' ').toLowerCase();
  if (planning && (group.includes('synth') || laneNames.includes('synth'))) {
    return { key: 'synthesis', label: 'Synthesis lane' };
  }
  if (planning) return { key: 'planning-lenses', label: 'Planning lenses' };
  return { key: 'execution', label: 'Execution phase' };
}

function renderSDDFlowGroup(flow, planning) {
  const role = sddFlowRole(flow, planning);
  const group = document.createElement('div');
  group.className = 'sdd-flow-group';
  group.setAttribute('data-flow-role', role.key);
  group.appendChild(createTextElement('div', 'sdd-flow-role', role.label));

  const meta = [];
  if (flow.fanout_group) meta.push(`Group: ${flow.fanout_group}`);
  if (flow.run_id) meta.push(`Run: ${flow.run_id}`);
  if (flow.status) meta.push(`Status: ${flow.status}`);
  if (Number.isInteger(flow.lane_count)) meta.push(`Lanes: ${flow.lane_count}`);
  if (meta.length > 0) group.appendChild(createTextElement('p', 'sdd-flow-meta', meta.join(' · ')));

  if (flow.lane_ids.length > 0) {
    const lanes = document.createElement('ul');
    lanes.className = 'sdd-flow-lanes';
    flow.lane_ids.forEach((laneID, index) => {
      let label = `Lane: ${laneID}`;
      if (role.key === 'planning-lenses') label = `Lens ${index + 1}: ${laneID}`;
      if (role.key === 'synthesis') label = `Synthesis: ${laneID}`;
      lanes.appendChild(createTextElement('li', '', label));
    });
    group.appendChild(lanes);
  }
  return group;
}

function renderSDDPhase(phase, flows, planning) {
  const phaseElement = document.createElement('section');
  phaseElement.className = 'sdd-phase';
  phaseElement.setAttribute('aria-label', `${phase.label} phase`);
  phaseElement.appendChild(createTextElement('h4', '', phase.label));

  if (!flows || flows.length === 0) {
    phaseElement.appendChild(createTextElement('p', 'sdd-phase-missing', 'Not reported by server'));
    return phaseElement;
  }
  flows.forEach(flow => phaseElement.appendChild(renderSDDFlowGroup(flow, planning)));
  return phaseElement;
}

function renderSDDRailSection(title, phases, flowsByPhase, planning) {
  const section = document.createElement('section');
  section.className = 'sdd-rail-section';
  section.appendChild(createTextElement('h3', 'sdd-rail-title', title));
  const rail = document.createElement('div');
  rail.className = 'sdd-rail';
  rail.setAttribute('aria-label', title);
  phases.forEach(phase => {
    rail.appendChild(renderSDDPhase(phase, flowsByPhase.get(phase.key), planning));
  });
  section.appendChild(rail);
  return section;
}

function renderSDDFlows(container, flows) {
  if (!container) return;
  while (container.children.length > 1) container.lastChild.remove();
  const changes = groupSDDFlowsByChange(Array.isArray(flows) ? flows : []);
  if (changes.size === 0) {
    container.appendChild(createTextElement('div', 'empty-state', 'No SDD flows reported.'));
    return;
  }

  changes.forEach((flowsByPhase, change) => {
    const changeElement = document.createElement('article');
    changeElement.className = 'approval-card sdd-change';
    changeElement.appendChild(createTextElement('h2', 'sdd-change-title', change));
    changeElement.appendChild(renderSDDRailSection('Planning rail', SDD_PLANNING_PHASES, flowsByPhase, true));
    changeElement.appendChild(renderSDDRailSection('Execution', SDD_EXECUTION_PHASES, flowsByPhase, false));
    container.appendChild(changeElement);
  });
}

function setupViewNavigation() {
  const controls = document.querySelectorAll('[data-view-target]');
  const outlets = document.querySelectorAll('[data-view-outlet]');
  controls.forEach(control => {
    control.addEventListener('click', () => {
      const target = control.dataset.viewTarget;
      controls.forEach(candidate => {
        candidate.setAttribute('aria-pressed', String(candidate === control));
      });
      outlets.forEach(outlet => {
        outlet.hidden = outlet.dataset.viewOutlet !== target;
      });
    });
  });
}

async function submitDecision(runID, laneID, decision) {
  if (!decision) return false;
  try {
    const response = await fetch(`/approvals/${encodeURIComponent(runID)}/${encodeURIComponent(laneID)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ decision })
    });
    if (!response.ok) {
      const detail = await response.text();
      window.alert(`Decision failed: ${detail}`);
      return false;
    }
    await controlRoomStore.refreshState('decision');
    return true;
  } catch (error) {
    console.error('Error submitting decision:', error);
    window.alert('Decision failed because the server could not be reached.');
    return false;
  }
}

async function submitDefect(runID, laneID, defect = true) {
  if (!runID || !laneID) return false;
  try {
    const response = await fetch(`/approvals/${encodeURIComponent(runID)}/${encodeURIComponent(laneID)}/defect`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ defect })
    });
    if (!response.ok) {
      const detail = await response.text();
      window.alert(`Defect update failed: ${detail}`);
      return false;
    }
    await controlRoomStore.refreshState('defect');
    return true;
  } catch (error) {
    console.error('Error submitting defect:', error);
    window.alert('Defect update failed because the server could not be reached.');
    return false;
  }
}

const controlRoomStore = createLiveStore();
controlRoomStore.subscribe(snapshot => {
  renderConnection(snapshot.connection);
  if (snapshot.state !== null) renderState(snapshot.state);
});
setupViewNavigation();
controlRoomStore.start();
window.addEventListener('pagehide', () => controlRoomStore.teardown(), { once: true });
