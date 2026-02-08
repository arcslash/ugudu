// WebSocket store for real-time updates

import { writable, get } from 'svelte/store';
import { teamMembers, addMessage, currentTeamName, wasRecentlyAdded, teams, specs, currentTeam, currentTeamName as currentTeamNameStore } from './app';
import { getTeams, getTeamMembers, getSpecs } from '../lib/api';

export interface WSEvent {
  type: 'member_status' | 'activity' | 'task_update' | 'chat' | 'team_update' | 'spec_update' | 'settings_update';
  team: string;
  member_id?: string;
  status?: string;
  message?: string;
  data?: any;
  timestamp: string;
}

// Connection state
export const wsConnected = writable(false);
export const wsLastEvent = writable<WSEvent | null>(null);

// Activity feed for the current team
export const activityFeed = writable<WSEvent[]>([]);

let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 10;
const RECONNECT_DELAY_BASE = 1000;

export function connectWebSocket() {
  if (ws && ws.readyState === WebSocket.OPEN) {
    return;
  }

  // Determine WebSocket URL based on current location
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/api/ws`;

  try {
    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('[WS] Connected');
      wsConnected.set(true);
      reconnectAttempts = 0;
    };

    ws.onmessage = (event) => {
      try {
        const data: WSEvent = JSON.parse(event.data);
        handleWSEvent(data);
      } catch (e) {
        console.error('[WS] Failed to parse message:', e);
      }
    };

    ws.onclose = () => {
      console.log('[WS] Disconnected');
      wsConnected.set(false);
      ws = null;
      scheduleReconnect();
    };

    ws.onerror = (error) => {
      console.error('[WS] Error:', error);
    };
  } catch (e) {
    console.error('[WS] Failed to connect:', e);
    scheduleReconnect();
  }
}

function scheduleReconnect() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
  }

  if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
    console.log('[WS] Max reconnect attempts reached');
    return;
  }

  const delay = RECONNECT_DELAY_BASE * Math.pow(2, reconnectAttempts);
  reconnectAttempts++;

  console.log(`[WS] Reconnecting in ${delay}ms (attempt ${reconnectAttempts})`);
  reconnectTimer = setTimeout(() => {
    connectWebSocket();
  }, delay);
}

function handleWSEvent(event: WSEvent) {
  wsLastEvent.set(event);
  console.log('[WS] Event received:', event.type, event.team || '', event.message || '');

  switch (event.type) {
    case 'member_status':
      handleMemberStatus(event);
      break;
    case 'activity':
      handleActivity(event);
      break;
    case 'task_update':
      // Future: handle task updates
      break;
    case 'chat':
      handleChat(event);
      break;
    case 'team_update':
      handleTeamUpdate(event);
      break;
    case 'spec_update':
      handleSpecUpdate(event);
      break;
    case 'settings_update':
      // Could refresh settings here if needed
      console.log('[WS] Settings updated:', event.message);
      break;
  }
}

function handleMemberStatus(event: WSEvent) {
  if (!event.member_id || !event.status) return;

  // Update member status in the store
  teamMembers.update(members => {
    return members.map(m => {
      // Match by role name (member_id from server is the role name)
      if (m.role === event.member_id || m.id === event.member_id) {
        return {
          ...m,
          status: event.status as 'idle' | 'busy' | 'working' | 'thinking' | 'error',
          statusMessage: event.message
        };
      }
      return m;
    });
  });
}

function handleActivity(event: WSEvent) {
  // Add to activity feed (keep last 50 events)
  activityFeed.update(feed => {
    const newFeed = [event, ...feed];
    return newFeed.slice(0, 50);
  });
}

function handleChat(event: WSEvent) {
  // Only process chat for the current team
  const teamName = get(currentTeamName);
  if (!teamName || event.team !== teamName) return;

  const memberId = event.member_id || '';
  const data = event.data || {};
  const msgType = data.msg_type === 'user' ? 'user' : 'agent';
  const from = data.from || (msgType === 'user' ? 'You' : memberId);
  const content = event.message || '';

  if (!content) return;

  // Skip if this message was recently added locally (prevents duplicates)
  if (wasRecentlyAdded(event.team, memberId, content)) {
    return;
  }

  // Add message to chat history
  addMessage(event.team, memberId, {
    from,
    content,
    type: msgType,
    role: memberId
  });
}

async function handleTeamUpdate(event: WSEvent) {
  const action = event.message; // created, deleted, started, stopped
  const teamName = event.team;

  console.log('[WS] Team update:', action, teamName);

  switch (action) {
    case 'created':
      // Refresh full team list to get new team details
      try {
        const teamList = await getTeams();
        teams.set(teamList);
      } catch (e) {
        console.error('Failed to refresh teams:', e);
      }
      break;

    case 'deleted':
      // Remove team from list
      teams.update(list => list.filter(t => t.name !== teamName));
      // Clear selection if this team was selected
      if (get(currentTeamName) === teamName) {
        currentTeamNameStore.set(null);
        currentTeam.set(null);
        teamMembers.set([]);
      }
      break;

    case 'started':
    case 'stopped':
      // Update team status
      teams.update(list =>
        list.map(t => t.name === teamName
          ? { ...t, status: action === 'started' ? 'running' as const : 'stopped' as const }
          : t
        )
      );
      break;
  }
}

async function handleSpecUpdate(event: WSEvent) {
  const action = event.message; // created, updated, deleted
  const specName = event.team; // Using team field for spec name

  console.log('[WS] Spec update:', action, specName);

  switch (action) {
    case 'created':
    case 'updated':
      // Refresh full spec list
      try {
        const specList = await getSpecs();
        specs.set(specList);
      } catch (e) {
        console.error('Failed to refresh specs:', e);
      }
      break;

    case 'deleted':
      // Remove spec from list
      specs.update(list => list.filter(s => s.name !== specName));
      break;
  }
}

export function disconnectWebSocket() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }

  if (ws) {
    ws.close();
    ws = null;
  }

  wsConnected.set(false);
}

// Auto-connect when module loads
if (typeof window !== 'undefined') {
  connectWebSocket();
}
