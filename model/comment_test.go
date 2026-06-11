package model

import "testing"

func TestIsValidCommentStatus(t *testing.T) {
	valid := []CommentStatus{CommentStatusPending, CommentStatusApproved, CommentStatusRejected, CommentStatusSpam}
	for _, status := range valid {
		if !IsValidCommentStatus(status) {
			t.Fatalf("IsValidCommentStatus(%d) = false, want true", status)
		}
	}
	invalid := []CommentStatus{CommentStatusLegacyApproved, CommentStatus(-1), CommentStatus(9)}
	for _, status := range invalid {
		if IsValidCommentStatus(status) {
			t.Fatalf("IsValidCommentStatus(%d) = true, want false", status)
		}
	}
}
