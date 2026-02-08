// WebSocket store for real-time updates

import { writable, get } from 'svelte/store';
import { teamMembers } from './app';

export interface WSEvent {
  type: 'member_status' | 'activity' | 'task_update';
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
