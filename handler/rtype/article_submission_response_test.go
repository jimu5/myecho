package rtype

import (
	"encoding/json"
	"strings"
	"testing"

	"myecho/model"
)

func TestModelToArticleSubmissionResponseHidesPasswordHash(t *testing.T) {
	const passwordHash = "protected-snapshot-hash"
	snapshot, err := json.Marshal(model.ArticleSubmissionSnapshot{Title: "Protected", PasswordHash: passwordHash})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	submission := &model.ArticleSubmission{Status: model.ArticleSubmissionDraft, Snapshot: string(snapshot)}

	response, err := ModelToArticleSubmissionResponse(submission)
	if err != nil {
		t.Fatalf("convert response: %v", err)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(encoded), passwordHash) || strings.Contains(string(encoded), `"password_hash"`) {
		t.Fatalf("response exposed stored password hash: %s", encoded)
	}
	if !response.Snapshot.IsPasswordProtected {
		t.Fatal("response lost password-protected state")
	}
	if response.Snapshot.ArticleSubmissionSnapshot.PasswordHash != "" {
		t.Fatal("response snapshot retained the stored password hash")
	}
	if !strings.Contains(submission.Snapshot, passwordHash) {
		t.Fatal("response conversion mutated the stored submission snapshot")
	}
}
