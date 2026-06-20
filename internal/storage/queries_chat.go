package storage

import (
	"context"
)

// EnsureConversationExists ensures that a conversation UUID exists in llm_conversations
// to satisfy the foreign key constraint for llm_messages. It's idempotent.
func (s *Storage) EnsureConversationExists(ctx context.Context, conversationID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO llm_conversations (id, created_at, updated_at) 
		VALUES ($1, NOW(), NOW())
		ON CONFLICT (id) DO NOTHING`, conversationID)
	return err
}

// SaveLLMMessage saves a single message (user or assistant) to the database.
func (s *Storage) SaveLLMMessage(ctx context.Context, msg LLMMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO llm_messages (conversation_id, role, content, created_at)
		VALUES ($1, $2, $3, NOW())`,
		msg.ConversationID, msg.Role, msg.Content)
	return err
}

// GetConversationHistory retrieves all messages for a given conversation,
// ordered chronologically.
func (s *Storage) GetConversationHistory(ctx context.Context, conversationID string) ([]LLMMessage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, conversation_id, role, content, created_at
		FROM llm_messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []LLMMessage
	for rows.Next() {
		var m LLMMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return msgs, nil
}
