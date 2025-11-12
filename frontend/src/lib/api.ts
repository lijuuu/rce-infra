const API_BASE_URL = 'http://localhost:8080/v1';

export interface Node {
  node_id: string;
  attrs: Record<string, any>;
  last_seen_at: string;
  disabled: boolean;
  is_healthy: boolean;
}

export interface Command {
  command_id: string;
  node_id: string;
  command_type: string;
  payload: Record<string, any>;
  status: string;
  exit_code?: number;
  error_msg?: string;
  created_at: string;
  updated_at: string;
}

export interface LogChunk {
  chunk_index: number;
  stream: string;
  data: string;
  is_final?: boolean; // true if this is the final chunk (work is done)
}

export interface CommandLogs {
  command_id: string;
  logs: LogChunk[];
}

export const api = {
  async listNodes(): Promise<Node[]> {
    const response = await fetch(`${API_BASE_URL}/agents`);
    if (!response.ok) throw new Error('Failed to fetch nodes');
    const data = await response.json();
    return data.nodes || [];
  },

  async listCommands(nodeId?: string, limit = 50): Promise<Command[]> {
    const params = new URLSearchParams();
    if (nodeId) params.append('node_id', nodeId);
    params.append('limit', limit.toString());
    const response = await fetch(`${API_BASE_URL}/commands?${params}`);
    if (!response.ok) throw new Error('Failed to fetch commands');
    const data = await response.json();
    return data.commands || [];
  },

  async submitCommand(nodeId: string, command: string, timeoutSec = 30): Promise<{ command_id: string }> {
    const response = await fetch(`${API_BASE_URL}/commands/submit`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        command_type: 'RunCommand',
        node_id: nodeId,
        payload: {
          cmd: command,
          timeout_sec: timeoutSec,
        },
      }),
    });
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to submit command');
    }
    return response.json();
  },

  async getCommandLogs(commandId: string, afterChunkIndex?: number): Promise<CommandLogs> {
    const url = afterChunkIndex !== undefined
      ? `${API_BASE_URL}/commands/${commandId}/logs?after_chunk_index=${afterChunkIndex}`
      : `${API_BASE_URL}/commands/${commandId}/logs`;
    const response = await fetch(url);
    if (!response.ok) throw new Error('Failed to fetch logs');
    return response.json();
  },
};

