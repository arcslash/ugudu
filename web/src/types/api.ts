// API Types for Ugudu

export interface Team {
  name: string;
  description?: string;
  status: 'running' | 'stopped';
  member_count: number;
  members: Member[];
  client_facing: string[];
  tasks: {
    total: number;
    pending: number;
    in_progress: number;
    completed: number;
  };
}

export interface Member {
  id: string;
  name: string;
  title: string;
  role: string;
  status: 'idle' | 'busy' | 'working' | 'thinking' | 'error';
  statusMessage?: string;
  client_facing: boolean;
  provider?: string;
  model?: string;
  task?: string;
}

export interface Spec {
  name: string;
  path?: string;
  members?: number;
  description?: string;
  provider?: string;
  model?: string;
  roles?: Record<string, SpecRole>;
  client_facing?: string[];
}

export interface SpecRole {
  title: string;
  name?: string;
  names?: string[];
  count?: number;
  persona?: string;
  provider?: string;  // Per-role provider override
  model?: string;     // Per-role model override
}

export interface ChatMessage {
  from: string;
  content: string;
  type: 'user' | 'agent';
  role?: string;
  timestamp?: Date;
}

export interface ChatResponse {
  responses?: Array<{
    from: string;
    content: string;
    type: string;
    role?: string;
  }>;
  response?: string;
  error?: string;
}

export interface Provider {
  id: string;
  name: string;
}
