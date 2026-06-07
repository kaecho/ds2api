package promptcompat

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"sync"
)

type contextFileVariant struct {
	HistoryFilename        string
	ToolsFilename          string
	HistoryTranscriptTitle string
	ToolsTranscriptTitle   string
	HistorySummary         string
	ToolsSummary           string
	InlinePromptText       string
	SectionSeparator       string
	SectionNumbering       string
	ContentType            string
	Purpose                string
}

var (
	variantOnce    sync.Once
	currentVariant contextFileVariant
)

var filenamePrefixes = []string{
	"context_", "history_", "chat_", "session_",
	"state_", "conversation_", "transcript_", "thread_",
}

var historySummaryVariants = []string{
	"Prior conversation history.",
	"Previous messages and state for continuity.",
	"Chat history context for continuation.",
	"Conversation transcript for reference.",
}

var toolsSummaryVariants = []string{
	"Available tool descriptions and parameter schemas for this request.",
	"Tool definitions and parameter schemas for function calling.",
	"Defined tools with their descriptions and parameter schemas.",
	"Callable function specifications and parameter schemas.",
}

var inlinePromptVariants = []string{
	"Continue from the latest state in the attached %s context. Treat it as the current working state and answer the latest user request directly.",
	"The attached file %s contains the prior conversation history. Pick up from the latest state and respond to the user's most recent message.",
	"Refer to the uploaded %s for the full conversation context. Continue from where the conversation left off and address the latest user input.",
	"Previous conversation context is provided in %s. Use it to maintain continuity and answer the current user request.",
}

var sectionSeparators = []string{"===", "---", "***"}

var sectionNumberings = []string{"%d.", "%d)", "[%d]"}

var contentTypes = []string{
	"text/plain; charset=utf-8",
	"text/plain",
}

var purposes = []string{"assistants", "user_data"}

func generateVariant() contextFileVariant {
	historyName := randomPrefix() + randomHex(8) + ".txt"
	toolsName := randomPrefix() + randomHex(8) + ".txt"

	v := contextFileVariant{
		HistoryFilename:        historyName,
		ToolsFilename:          toolsName,
		HistoryTranscriptTitle: "# " + historyName,
		ToolsTranscriptTitle:   "# " + toolsName,
		HistorySummary:         pickString(historySummaryVariants),
		ToolsSummary:           pickString(toolsSummaryVariants),
		InlinePromptText:       pickString(inlinePromptVariants),
		SectionSeparator:       pickString(sectionSeparators),
		SectionNumbering:       pickString(sectionNumberings),
		ContentType:            pickString(contentTypes),
		Purpose:                pickString(purposes),
	}
	return v
}

func GetCurrentVariant() contextFileVariant {
	variantOnce.Do(func() {
		currentVariant = generateVariant()
	})
	return currentVariant
}

func CurrentInputContextFilename() string {
	return GetCurrentVariant().HistoryFilename
}

func CurrentToolsContextFilename() string {
	return GetCurrentVariant().ToolsFilename
}

func ResetVariantForTest() {
	variantOnce = sync.Once{}
}

func pickString(s []string) string {
	if len(s) == 0 {
		return ""
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(s))))
	if err != nil {
		return s[0]
	}
	return s[n.Int64()]
}

func randomHex(n int) string {
	b := make([]byte, n/2+1)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

func randomPrefix() string {
	return pickString(filenamePrefixes)
}
