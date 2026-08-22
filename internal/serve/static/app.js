'use strict';

const POLL_INTERVAL_MS = 2000;

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

function approvalKey(item) {
  return `${item.RunID}\u0000${item.LaneID}`;
}

function createApprovalCard(item) {
  const card = document.createElement('article');
  card.className = 'approval-card';
  card.setAttribute('data-approval-key', approvalKey(item));

  const header = document.createElement('div');
  header.className = 'card-header';
  const lane = document.createElement('span');
  lane.className = 'lane-id';
  const packet = document.createElement('span');
  packet.className = 'packet-id';
  header.append(lane, packet);

  const evidenceLabel = document.createElement('div');
  evidenceLabel.className = 'evidence-label';
  evidenceLabel.textContent = 'Approval evidence';
  const evidence = document.createElement('pre');
  evidence.className = 'evidence-block';
  const noEvidence = document.createElement('div');
  noEvidence.className = 'no-evidence';
  noEvidence.textContent = 'No command output or file:line evidence provided.';

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
  actions.append(approveButton, rejectButton);

  async function decide(decision) {
    approveButton.disabled = true;
    rejectButton.disabled = true;
    const accepted = await submitDecision(item.RunID, item.LaneID, decision);
    if (!accepted) {
      approveButton.disabled = false;
      rejectButton.disabled = false;
    }
  }

  approveButton.addEventListener('click', () => void decide('approved'));
  rejectButton.addEventListener('click', () => void decide('rejected'));
  card.append(header, evidenceLabel, evidence, noEvidence, actions);
  card._approvalParts = { lane, packet, evidence, noEvidence };
  updateApprovalCard(card, item);
  return card;
}

function updateApprovalCard(card, item) {
  const parts = card._approvalParts;
  parts.lane.textContent = `Lane: ${item.LaneID || '-'}`;
  parts.packet.textContent = `Packet: ${item.PacketID || '-'}`;
  const evidenceValid = isValidEvidence(item.Evidence);
  parts.evidence.textContent = evidenceValid ? item.Evidence : '';
  parts.evidence.hidden = !evidenceValid;
  parts.noEvidence.hidden = evidenceValid;
}

function patchApprovalCards(approvalsContainer, approvals) {
  const pending = approvals.filter(item => item.Decision === 'pending');
  const currentCards = new Map();
  approvalsContainer.querySelectorAll('[data-approval-key]').forEach(card => {
    currentCards.set(card.getAttribute('data-approval-key'), card);
  });

  const activeKeys = new Set();
  pending.forEach(item => {
    const key = approvalKey(item);
    activeKeys.add(key);
    const card = currentCards.get(key);
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
  if (pending.length === 0 && emptyState === null) {
    emptyState = document.createElement('div');
    emptyState.className = 'empty-state';
    emptyState.setAttribute('data-empty-state', '');
    emptyState.textContent = 'No pending approvals.';
    approvalsContainer.appendChild(emptyState);
  } else if (pending.length > 0 && emptyState !== null) {
    emptyState.remove();
  }

  return pending.length;
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
  if (fleetContainer) patchFleetCards(fleetContainer, normalizeFleetState(state, Date.now()));
  renderApplyDAG(state.apply_dag);
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

const controlRoomStore = createLiveStore();
controlRoomStore.subscribe(snapshot => {
  renderConnection(snapshot.connection);
  if (snapshot.state !== null) renderState(snapshot.state);
});
setupViewNavigation();
controlRoomStore.start();
window.addEventListener('pagehide', () => controlRoomStore.teardown(), { once: true });
