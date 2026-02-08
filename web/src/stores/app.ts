// Svelte stores for app state

import { writable, derived } from 'svelte/store';
import type { Team, Member, Spec, ChatMessage } from '../types/api';

// Current view
export type View = 'dashboard' | 'specs' | 'providers' | 'team';
export const currentView = writable<View>('dashboard');

// Teams
export const teams = writable<Team[]>([]);
export const currentTeam = writable<Team | null>(null);
export const currentTeamName = writable<string | null>(null);

// Members
export const teamMembers = writable<Member[]>([]);
export const selectedMember = writable<Member | null>(null);

// Derived: client-facing members only
export const clientFacingMembers = derived(teamMembers, $members =>
  $members.filter(m => m.client_facing)
);

// Chat history per team:member
export const chatHistory = writable<Record<string, ChatMessage[]>>({});

// Get/set chat for current context
export function getChatKey(teamName: string, memberId: string): string {
  return `${teamName}:${memberId}`;
}

// Specs
export const specs = writable<Spec[]>([]);

// UI State
export const isLoading = writable(false);
export const error = writable<string | null>(null);
export const showCreateTeamModal = writable(false);
export const showSpecEditorModal = writable(false);
export const editingSpec = writable<string | null>(null);

// Persist chat history to localStorage
const STORAGE_KEY = 'ugudu_chat_history';

export function loadChatHistory() {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      chatHistory.set(JSON.parse(saved));
    }
  } catch (e) {
    console.error('Failed to load chat history:', e);
  }
}

export function saveChatHistory() {
  chatHistory.subscribe(value => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
    } catch (e) {
      console.error('Failed to save chat history:', e);
    }
  })();
}

export function addMessage(teamName: string, memberId: string, message: ChatMessage) {
  const key = getChatKey(teamName, memberId);
  chatHistory.update(history => {
    const messages = history[key] || [];
    return {
      ...history,
      [key]: [...messages, { ...message, timestamp: new Date() }]
    };
  });
  saveChatHistory();
}

export function clearChat(teamName: string, memberId: string) {
  const key = getChatKey(teamName, memberId);
  chatHistory.update(history => {
    const { [key]: _, ...rest } = history;
    return rest;
  });
  saveChatHistory();
}
