package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"docket/internal/store"
)

func TestQuestionDetailFormatting(t *testing.T) {
	q := store.Question{ID: 22, Status: store.QOpen,
		Source: "11. Q&A/1.1.0/ISI213_1.1.0_po_answers.md câu 11; ADR-029; Integration Contracts §4 (không gửi email, tên)",
		Text: "Câu giải thích plan muốn mở đầu bằng tên người dùng, nhưng ADR-029 cấm gửi tên sang dịch vụ AI. " +
			"Chọn: (A) giữ cấm, BE tự ghép tên vào câu mở đầu (AI trả mẫu có chỗ {name}); (B) cho phép gửi tên sang dịch vụ AI. Đề xuất A"}
	out := ansi.Strip(questionDetail(q, false))
	for _, want := range []string{
		"  • ADR-029\n",
		"\n  (A) giữ cấm",
		"\n  (B) cho phép",
		"→ Đề xuất: A",
		"(không gửi email, tên)", // parentheses that are not options stay inline
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
