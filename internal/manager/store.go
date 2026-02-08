package manager

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store handles persistence for the manager
type Store struct {
	db *sql.DB
}

// NewStore creates a new store
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Set connection limits for SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS teams (
			name TEXT PRIMARY KEY,
			spec_path TEXT NOT NULL,
			status TEXT DEFAULT 'stopped',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS team_messages (
			id TEXT PRIMARY KEY,
			team_name TEXT NOT NULL,
			type TEXT NOT NULL,
			from_member TEXT,
			to_member TEXT,
			content TEXT,
			task_id TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (team_name) REFERENCES teams(name)
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			team_name TEXT NOT NULL,
			content TEXT NOT NULL,
			from_member TEXT,
			to_member TEXT,
			status TEXT DEFAULT 'pending',
			priority INTEGER DEFAULT 0,
			result TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			FOREIGN KEY (team_name) REFERENCES teams(name)
		)`,
		// Conversations table - groups related messages into sessions
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			team_name TEXT NOT NULL,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_message_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT DEFAULT 'active',
			metadata TEXT,
			FOREIGN KEY (team_name) REFERENCES teams(name)
		)`,
		// Agent conversation context - stores LLM message history per member
		`CREATE TABLE IF NOT EXISTS agent_context (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			team_name TEXT NOT NULL,
			member_id TEXT NOT NULL,
			conversation_id TEXT,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (team_name) REFERENCES teams(name),
			FOREIGN KEY (conversation_id) REFERENCES conversations(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_team ON tasks(team_name)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_team ON team_messages(team_name)`,
		`CREATE INDEX IF NOT EXISTS idx_conversations_team ON conversations(team_name)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_context_member ON agent_context(team_name, member_id)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_context_conv ON agent_context(conversation_id)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	return nil
}

// SaveTeam persists a team
func (s *Store) SaveTeam(name, specPath string) error {
	_, err := s.db.Exec(`
		INSERT INTO teams (name, spec_path, status)
		VALUES (?, ?, 'stopped')
		ON CONFLICT(name) DO UPDATE SET
			spec_path = excluded.spec_path,
			updated_at = CURRENT_TIMESTAMP
	`, name, specPath)
	return err
}

// UpdateTeamStatus updates a team's status
func (s *Store) UpdateTeamStatus(name, status string) error {
	_, err := s.db.Exec(`
		UPDATE teams SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE name = ?
	`, status, name)
	return err
}

// DeleteTeam removes a team from the store
func (s *Store) DeleteTeam(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete related data first
	if _, err := tx.Exec(`DELETE FROM tasks WHERE team_name = ?`, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM team_messages WHERE team_name = ?`, name); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM teams WHERE name = ?`, name); err != nil {
		return err
	}

	return tx.Commit()
}

// GetTeam retrieves a team by name
func (s *Store) GetTeam(name string) (*SavedTeam, error) {
	var team SavedTeam
	err := s.db.QueryRow(`
		SELECT name, spec_path, status FROM teams WHERE name = ?
	`, name).Scan(&team.Name, &team.SpecPath, &team.Status)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &team, nil
}

// ListTeams returns all saved teams
func (s *Store) ListTeams() ([]SavedTeam, error) {
	rows, err := s.db.Query(`
		SELECT name, spec_path, status FROM teams ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []SavedTeam
	for rows.Next() {
		var team SavedTeam
		if err := rows.Scan(&team.Name, &team.SpecPath, &team.Status); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}

	return teams, rows.Err()
}

// SaveMessage persists a team message
func (s *Store) SaveMessage(teamName string, msg map[string]interface{}) error {
	_, err := s.db.Exec(`
		INSERT INTO team_messages (id, team_name, type, from_member, to_member, content, task_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		msg["id"],
		teamName,
		msg["type"],
		msg["from"],
		msg["to"],
		msg["content"],
		msg["task_id"],
	)
	return err
}

// SaveTask persists a task
func (s *Store) SaveTask(teamName string, task map[string]interface{}) error {
	_, err := s.db.Exec(`
		INSERT INTO tasks (id, team_name, content, from_member, to_member, status, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			completed_at = CASE WHEN excluded.status IN ('completed', 'failed') THEN CURRENT_TIMESTAMP ELSE NULL END
	`,
		task["id"],
		teamName,
		task["content"],
		task["from"],
		task["to"],
		task["status"],
		task["priority"],
	)
	return err
}

// GetRecentMessages retrieves recent messages for a team
func (s *Store) GetRecentMessages(teamName string, limit int) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT id, type, from_member, to_member, content, task_id, timestamp
		FROM team_messages
		WHERE team_name = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, teamName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []map[string]interface{}
	for rows.Next() {
		var id, msgType, content string
		var fromMember, toMember, taskID sql.NullString
		var timestamp time.Time

		if err := rows.Scan(&id, &msgType, &fromMember, &toMember, &content, &taskID, &timestamp); err != nil {
			return nil, err
		}

		msg := map[string]interface{}{
			"id":        id,
			"type":      msgType,
			"content":   content,
			"timestamp": timestamp,
		}
		if fromMember.Valid {
			msg["from"] = fromMember.String
		}
		if toMember.Valid {
			msg["to"] = toMember.String
		}
		if taskID.Valid {
			msg["task_id"] = taskID.String
		}

		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// GetTasks retrieves tasks for a team
func (s *Store) GetTasks(teamName string, status string) ([]map[string]interface{}, error) {
	query := `
		SELECT id, content, from_member, to_member, status, priority, result, created_at, completed_at
		FROM tasks
		WHERE team_name = ?
	`
	args := []interface{}{teamName}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}

	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []map[string]interface{}
	for rows.Next() {
		var id, content, taskStatus string
		var fromMember, toMember, result sql.NullString
		var priority int
		var createdAt time.Time
		var completedAt sql.NullTime

		if err := rows.Scan(&id, &content, &fromMember, &toMember, &taskStatus, &priority, &result, &createdAt, &completedAt); err != nil {
			return nil, err
		}

		task := map[string]interface{}{
			"id":         id,
			"content":    content,
			"status":     taskStatus,
			"priority":   priority,
			"created_at": createdAt,
		}
		if fromMember.Valid {
			task["from"] = fromMember.String
		}
		if toMember.Valid {
			task["to"] = toMember.String
		}
		if result.Valid {
			task["result"] = result.String
		}
		if completedAt.Valid {
			task["completed_at"] = completedAt.Time
		}

		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// ============================================================================
// Conversation Persistence
// ============================================================================

// Conversation represents a chat session
type Conversation struct {
	ID            string    `json:"id"`
	TeamName      string    `json:"team_name"`
	StartedAt     time.Time `json:"started_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	Status        string    `json:"status"`
}

// AgentMessage represents a message in an agent's LLM context
type AgentMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// CreateConversation starts a new conversation session
func (s *Store) CreateConversation(teamName string) (*Conversation, error) {
	id := fmt.Sprintf("conv-%d", time.Now().UnixNano())
	now := time.Now()

	_, err := s.db.Exec(`
		INSERT INTO conversations (id, team_name, started_at, last_message_at, status)
		VALUES (?, ?, ?, ?, 'active')
	`, id, teamName, now, now)

	if err != nil {
		return nil, err
	}

	return &Conversation{
		ID:            id,
		TeamName:      teamName,
		StartedAt:     now,
		LastMessageAt: now,
		Status:        "active",
	}, nil
}

// GetActiveConversation returns the most recent active conversation for a team
func (s *Store) GetActiveConversation(teamName string) (*Conversation, error) {
	var conv Conversation
	err := s.db.QueryRow(`
		SELECT id, team_name, started_at, last_message_at, status
		FROM conversations
		WHERE team_name = ? AND status = 'active'
		ORDER BY last_message_at DESC
		LIMIT 1
	`, teamName).Scan(&conv.ID, &conv.TeamName, &conv.StartedAt, &conv.LastMessageAt, &conv.Status)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &conv, nil
}

// UpdateConversationTimestamp updates the last message time
func (s *Store) UpdateConversationTimestamp(conversationID string) error {
	_, err := s.db.Exec(`
		UPDATE conversations SET last_message_at = CURRENT_TIMESTAMP WHERE id = ?
	`, conversationID)
	return err
}

// CloseConversation marks a conversation as closed
func (s *Store) CloseConversation(conversationID string) error {
	_, err := s.db.Exec(`
		UPDATE conversations SET status = 'closed' WHERE id = ?
	`, conversationID)
	return err
}

// SaveAgentContext saves a message to an agent's conversation context
func (s *Store) SaveAgentContext(teamName, memberID, conversationID, role, content string, sequence int) error {
	_, err := s.db.Exec(`
		INSERT INTO agent_context (team_name, member_id, conversation_id, role, content, sequence)
		VALUES (?, ?, ?, ?, ?, ?)
	`, teamName, memberID, conversationID, role, content, sequence)
	return err
}

// LoadAgentContext retrieves the conversation context for a member
func (s *Store) LoadAgentContext(teamName, memberID string, limit int) ([]AgentMessage, error) {
	// Get messages from the most recent conversation
	rows, err := s.db.Query(`
		SELECT role, content
		FROM agent_context
		WHERE team_name = ? AND member_id = ?
		AND conversation_id = (
			SELECT id FROM conversations
			WHERE team_name = ? AND status = 'active'
			ORDER BY last_message_at DESC
			LIMIT 1
		)
		ORDER BY sequence ASC
		LIMIT ?
	`, teamName, memberID, teamName, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []AgentMessage
	for rows.Next() {
		var msg AgentMessage
		if err := rows.Scan(&msg.Role, &msg.Content); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// ClearAgentContext removes old context for a member (for context window management)
func (s *Store) ClearAgentContext(teamName, memberID string) error {
	_, err := s.db.Exec(`
		DELETE FROM agent_context WHERE team_name = ? AND member_id = ?
	`, teamName, memberID)
	return err
}

// GetConversationHistory returns all messages from a conversation
func (s *Store) GetConversationHistory(conversationID string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		SELECT member_id, role, content, sequence, created_at
		FROM agent_context
		WHERE conversation_id = ?
		ORDER BY sequence ASC
	`, conversationID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []map[string]interface{}
	for rows.Next() {
		var memberID, role, content string
		var sequence int
		var createdAt time.Time

		if err := rows.Scan(&memberID, &role, &content, &sequence, &createdAt); err != nil {
			return nil, err
		}

		messages = append(messages, map[string]interface{}{
			"member_id":  memberID,
			"role":       role,
			"content":    content,
			"sequence":   sequence,
			"created_at": createdAt,
		})
	}

	return messages, rows.Err()
}

// ListConversations returns recent conversations for a team
func (s *Store) ListConversations(teamName string, limit int) ([]Conversation, error) {
	rows, err := s.db.Query(`
		SELECT id, team_name, started_at, last_message_at, status
		FROM conversations
		WHERE team_name = ?
		ORDER BY last_message_at DESC
		LIMIT ?
	`, teamName, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conv Conversation
		if err := rows.Scan(&conv.ID, &conv.TeamName, &conv.StartedAt, &conv.LastMessageAt, &conv.Status); err != nil {
			return nil, err
		}
		conversations = append(conversations, conv)
	}

	return conversations, rows.Err()
}
