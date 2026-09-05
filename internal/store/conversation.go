package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	ar "github.com/z-chenhao/ad-agent/internal/runtime"
)

// SelectExecution changes execution ownership, not business-session identity.
// Native checkpoints are never transferred between runtimes or model connections.
func (s *Session) SelectExecution(runtime string, requested, fallback ar.ModelSelection) (bool, error) {
	selection := requested
	if selection == (ar.ModelSelection{}) {
		selection = fallback
		if s.Model != (ar.ModelSelection{}) {
			selection = s.Model
		}
	}
	selection, err := ar.NormalizeModel(selection)
	if err != nil {
		return false, err
	}
	if runtime == "codex" {
		if err := ar.ValidateCodexModel(selection); err != nil {
			return false, err
		}
	}
	if runtime == "claude" && (selection.AuthMode != ar.APIKeyAuth || selection.Provider != "anthropic" || selection.API != ar.AnthropicMessages) {
		return false, errors.New("claude_requires_anthropic_messages")
	}
	changed := s.Runtime != runtime || s.Model != selection
	unsettled := len(s.Messages) > 0 && s.Messages[len(s.Messages)-1].Status != "completed"
	if changed || unsettled {
		s.Checkpoint = ""
	}
	s.Runtime, s.Model = runtime, selection
	return changed, nil
}

const ConversationLimit = 24000

// Native history is valid only for the system instructions and tool catalog
// that produced it. Deployment/skill changes rebuild public context before the
// runtime starts, without sending an incompatible checkpoint or retrying tools.
func (s *Session) BindExecutionContract(system string, tools []ar.Tool) {
	data, _ := json.Marshal(struct {
		System string
		Tools  []ar.Tool
	}{system, tools})
	digest := sha256.Sum256(data)
	next := hex.EncodeToString(digest[:])
	if s.ExecutionContract != next {
		s.Checkpoint = ""
	}
	s.ExecutionContract = next
}

type ConversationPage struct {
	Turns            []ConversationTurn `json:"turns"`
	NextBeforeTurnID string             `json:"next_before_turn_id,omitempty"`
}

type ConversationTurn struct {
	ID        string            `json:"turn_id"`
	Messages  []Message         `json:"messages"`
	Context   json.RawMessage   `json:"context,omitempty"`
	Cards     []json.RawMessage `json:"cards,omitempty"`
	Tools     []HistoricalTool  `json:"tools,omitempty"`
	Truncated bool              `json:"truncated,omitempty"`
}

type HistoricalTool struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Conversation projects only application-owned public records. Native transcripts,
// private reasoning, credentials and turn-local dataset authority never enter it.
// before is exclusive and must belong to this already source-bound session.
func (s *Store) Conversation(ctx context.Context, session Session, before string) (ConversationPage, error) {
	ids := []string{}
	byTurn := map[string][]Message{}
	for _, message := range session.Messages {
		if _, ok := byTurn[message.TurnID]; !ok {
			ids = append(ids, message.TurnID)
		}
		byTurn[message.TurnID] = append(byTurn[message.TurnID], message)
	}
	end := len(ids)
	if before != "" {
		end = -1
		for i, id := range ids {
			if id == before {
				end = i
				break
			}
		}
		if end < 0 {
			return ConversationPage{}, errors.New("conversation_cursor_not_in_session")
		}
	}
	page := ConversationPage{Turns: []ConversationTurn{}}
	for i := end - 1; i >= 0; i-- {
		turn := ConversationTurn{ID: ids[i]}
		for _, message := range byTurn[ids[i]] {
			if message.Role != "user" && message.Role != "assistant" {
				continue
			}
			runes := []rune(message.Text)
			if len(runes) > 2000 {
				message.Text = string(runes[:2000])
				turn.Truncated = true
			}
			turn.Messages = append(turn.Messages, message)
		}
		events, err := s.Events(ctx, ids[i], 0)
		if err != nil {
			return ConversationPage{}, err
		}
		for _, event := range events {
			switch event.Type {
			case "context.bound":
				if len(event.Data) <= 2500 {
					turn.Context = event.Data
				} else {
					turn.Truncated = true
				}
			case "ui.upsert":
				if len(event.Data) <= 4000 && len(turn.Cards) < 3 {
					turn.Cards = append(turn.Cards, event.Data)
				} else {
					turn.Truncated = true
				}
			case "tool.finished":
				var tool HistoricalTool
				if json.Unmarshal(event.Data, &tool) == nil && len(turn.Tools) < 12 {
					turn.Tools = append(turn.Tools, tool)
				} else {
					turn.Truncated = true
				}
			}
		}
		// Keep at least a readable turn even when its cards dominate the budget.
		encoded, _ := json.Marshal(turn)
		if len(encoded) > ConversationLimit/2 {
			turn.Cards, turn.Tools, turn.Context, turn.Truncated = nil, nil, nil, true
		}
		for encoded, _ = json.Marshal(turn); len(encoded) > ConversationLimit/2; encoded, _ = json.Marshal(turn) {
			for j := range turn.Messages {
				runes := []rune(turn.Messages[j].Text)
				turn.Messages[j].Text = string(runes[:len(runes)/2])
			}
			turn.Truncated = true
		}
		candidate := ConversationPage{Turns: append([]ConversationTurn{turn}, page.Turns...)}
		if i > 0 {
			candidate.NextBeforeTurnID = ids[i]
		}
		encoded, _ = json.Marshal(candidate)
		if len(encoded) > ConversationLimit || len(candidate.Turns) > 6 {
			break
		}
		page = candidate
	}
	return page, nil
}
