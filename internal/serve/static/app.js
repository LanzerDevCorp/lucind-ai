async function fetchState() {
  try {
    const res = await fetch('/api/state');
    if (!res.ok) return;
    const state = await res.json();
    renderState(state);
  } catch (err) {
    console.error('Failed to fetch state:', err);
  }
}

function isValidEvidence(ev) {
  if (!ev || typeof ev !== 'string') return false;
  const trimmed = ev.trim();
  if (!trimmed) return false;
  // Evidence must be command output or file:line reference
  const hasFileLine = /[\w\-./]+\.\w+:\d+/.test(trimmed);
  const hasCommandOutput = trimmed.startsWith('ok ') || trimmed.startsWith('PASS') || trimmed.startsWith('$ ') || trimmed.includes('---');
  return hasFileLine || hasCommandOutput;
}

function renderState(state) {
  const approverEl = document.getElementById('approver-name');
  const rateEl = document.getElementById('approver-rate');
  const cmdEl = document.getElementById('opencode-cmd');
  const containerEl = document.getElementById('approvals-container');

  if (approverEl) approverEl.textContent = state.approver || '-';
  if (rateEl) {
    const pct = ((state.approver_rate || 0) * 100).toFixed(1);
    rateEl.textContent = `${pct}%`;
  }
  if (cmdEl && state.opencode_command) {
    cmdEl.textContent = state.opencode_command;
  }

  if (!containerEl) return;

  const pending = (state.approvals || []).filter(a => a.Decision === 'pending');
  if (pending.length === 0) {
    containerEl.innerHTML = '<div class="empty-state">No pending approvals.</div>';
    return;
  }

  containerEl.innerHTML = '';
  pending.forEach(item => {
    const card = document.createElement('div');
    card.className = 'approval-card';
    card.id = `card-${item.RunID}-${item.LaneID}`;

    const evidenceValid = isValidEvidence(item.Evidence);
    const evidenceHtml = evidenceValid
      ? `<pre class="evidence-block">${escapeHtml(item.Evidence)}</pre>`
      : `<div class="no-evidence">(no command output or file:line evidence provided)</div>`;

    card.innerHTML = `
      <div class="card-header">
        <span class="lane-id">Lane: ${escapeHtml(item.LaneID)}</span>
        <span class="packet-id">Packet: ${escapeHtml(item.PacketID)}</span>
      </div>
      <div><strong>Evidence:</strong></div>
      ${evidenceHtml}
      <div class="card-actions">
        <button class="btn-approve" onclick="submitDecision('${item.RunID}', '${item.LaneID}', 'approved')">Approve</button>
        <button class="btn-reject" onclick="submitDecision('${item.RunID}', '${item.LaneID}', 'rejected')">Reject</button>
      </div>
    `;
    containerEl.appendChild(card);
  });
}

async function submitDecision(runID, laneID, decision) {
  if (!decision) return;
  try {
    const res = await fetch(`/approvals/${encodeURIComponent(runID)}/${encodeURIComponent(laneID)}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ decision })
    });
    if (res.ok) {
      fetchState();
    } else {
      const errText = await res.text();
      alert(`Decision failed: ${errText}`);
    }
  } catch (err) {
    console.error('Error submitting decision:', err);
  }
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

fetchState();
setInterval(fetchState, 2000);
