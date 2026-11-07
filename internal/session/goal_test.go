package session

import (
	"encoding/json"
	"testing"
)

func goalEntry(t *testing.T, customType, data string) LogEntry {
	t.Helper()
	if !json.Valid([]byte(data)) {
		t.Fatalf("bad fixture: %s", data)
	}
	return LogEntry{Type: "custom", CustomType: customType, Data: json.RawMessage(data)}
}

func TestReadGoalUsesNewestSetAndHonorsClear(t *testing.T) {
	budget := int64(200000)
	entries := []LogEntry{
		goalEntry(t, GoalEntryType, `{"version":1,"kind":"set","goal":{"goalId":"g1","objective":"old","status":"paused"}}`),
		goalEntry(t, GoalEntryType, `{"version":1,"kind":"set","goal":{"goalId":"g2","objective":"ship parity","status":"active","tokenBudget":200000,"usage":{"tokensUsed":1234}}}`),
		goalEntry(t, GoalEntryType, `{"version":1,"kind":"continuation-state","state":{}}`),
	}
	goal, ok := ReadGoal(entries)
	if !ok {
		t.Fatal("goal not found")
	}
	if goal.ID != "g2" || goal.Objective != "ship parity" || goal.Status != "active" {
		t.Fatalf("goal = %+v", goal)
	}
	if !goal.HasTokens || goal.TokensUsed != 1234 || goal.TokenBudget == nil || *goal.TokenBudget != budget {
		t.Fatalf("usage = %+v", goal)
	}

	entries = append(entries, goalEntry(t, GoalEntryType, `{"version":1,"kind":"clear","clearedGoalId":"g2"}`))
	if _, ok := ReadGoal(entries); ok {
		t.Fatal("cleared goal still present")
	}
}

func TestReadGoalReadsLegacyEntries(t *testing.T) {
	entries := []LogEntry{
		goalEntry(t, LegacyGoalEntryType, `{"kind":"set","goal":{"objective":"legacy","status":"budgetLimited"}}`),
	}
	goal, ok := ReadGoal(entries)
	if !ok || goal.Objective != "legacy" || GoalPhase(goal.Status) != "waiting" {
		t.Fatalf("goal = %+v, %v", goal, ok)
	}
}

func TestReadGoalIgnoresUnreadableRecords(t *testing.T) {
	entries := []LogEntry{
		goalEntry(t, GoalEntryType, `{"kind":"set","goal":{"status":"active","objective":"good"}}`),
		goalEntry(t, GoalEntryType, `{"version":1,"kind":"set"}`),
		goalEntry(t, "someone-else", `{"kind":"set","goal":{"status":"active","objective":"other"}}`),
	}
	goal, ok := ReadGoal(entries)
	if !ok || goal.Objective != "good" {
		t.Fatalf("goal = %+v, %v", goal, ok)
	}
}
