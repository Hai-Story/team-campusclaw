package tutor

import "time"

// UNIX_TIMESTAMP interprets MySQL TIMESTAMP in its session timezone and yields
// an absolute instant, avoiding ambiguity when serializing legacy rows.
const conversationTimeColumns = `c.id,c.agent_id,a.name,(a.deleted_at IS NOT NULL),c.title,
 UNIX_TIMESTAMP(c.created_at),
 UNIX_TIMESTAMP(GREATEST(c.updated_at,COALESCE((SELECT MAX(m.created_at) FROM messages m WHERE m.conversation_id=c.id),c.updated_at))) AS last_activity`

func timestamp(seconds int64) string { return time.Unix(seconds, 0).UTC().Format(time.RFC3339) }

func scanConversation(row interface{ Scan(...any) error }) (Conversation, error) {
	var c Conversation
	var created, updated int64
	err := row.Scan(&c.ID, &c.AgentID, &c.AgentName, &c.AgentDeleted, &c.Title, &created, &updated)
	if err == nil {
		c.Created, c.Updated = timestamp(created), timestamp(updated)
	}
	return c, err
}
