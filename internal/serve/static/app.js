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

function renderConnection(connection) {
  const status = document.getElementById('connection-status');
  const text = document.getElementById('connection-status-text');
  if (!status || !text) return;
  status.dataset.mode = connection.mode;
  text.textContent = connection.message;
}

function renderState(state) {
  const approver = document.getElementById('approver-name');
  const rate = document.getElementById('approver-rate');
  const command = document.getElementById('opencode-cmd');
  const pendingCount = document.getElementById('pending-approvals-count');
  const approvalsContainer = document.getElementById('approvals-container');

  if (approver) approver.textContent = state.approver || '-';
  if (rate) rate.textContent = `${((state.approver_rate || 0) * 100).toFixed(1)}%`;
  if (command && state.opencode_command) command.textContent = state.opencode_command;
  if (approvalsContainer) {
    const count = patchApprovalCards(approvalsContainer, state.approvals || []);
    if (pendingCount) pendingCount.textContent = String(count);
  }
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
