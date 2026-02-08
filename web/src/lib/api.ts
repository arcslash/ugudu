// Ugudu API Client

import type { Team, Spec, ChatResponse, Provider } from '../types/api';

const API_BASE = '/api';

async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${endpoint}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }

  return res.json();
}

// Teams
export async function getTeams(): Promise<Team[]> {
  const data = await request<{ teams: Team[] }>('/teams');
  return data.teams || [];
}

export async function getTeamMembers(teamName: string) {
  return request<{ members: any[] }>(`/teams/${teamName}/members`);
}

export async function createTeam(name: string, spec: string) {
  return request('/teams', {
    method: 'POST',
    body: JSON.stringify({ name, spec })
  });
}

export async function startTeam(name: string) {
  return request(`/teams/${name}/start`, { method: 'POST' });
}

export async function stopTeam(name: string) {
  return request(`/teams/${name}/stop`, { method: 'POST' });
}

export async function deleteTeam(name: string) {
  return request(`/teams/${name}`, { method: 'DELETE' });
}

// Chat
export async function sendMessage(team: string, message: string, to?: string): Promise<ChatResponse> {
  return request('/chat', {
    method: 'POST',
    body: JSON.stringify({ team, message, to })
  });
}

// Specs
export async function getSpecs(): Promise<Spec[]> {
  const data = await request<{ specs: Spec[] }>('/specs');
  return data.specs || [];
}

export async function getSpec(name: string): Promise<Spec> {
  return request(`/specs/${name}`);
}

export async function saveSpec(spec: {
  name: string;
  description?: string;
  provider: string;
  model: string;
  roles: Record<string, any>;
  client_facing: string[];
}) {
  return request('/specs', {
    method: 'POST',
    body: JSON.stringify(spec)
  });
}

export async function deleteSpec(name: string) {
  return request(`/specs/${name}`, { method: 'DELETE' });
}

// Providers
export async function getProviders(): Promise<Provider[]> {
  const data = await request<{ providers: Provider[] }>('/providers');
  return data.providers || [];
}

export async function testProvider(id: string) {
  return request(`/providers/${id}/test`);
}
