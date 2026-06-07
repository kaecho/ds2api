package shared

import textclean "ds2api/internal/textclean"

func CleanVisibleOutput(text string, stripReferenceMarkers bool) string {
	return CleanVisibleOutputWithPolicy(text, stripReferenceMarkers, false)
}

func CleanVisibleOutputWithPolicy(text string, stripReferenceMarkers bool, preserveToolMarkup bool) string {
	if text == "" {
		return text
	}
	if stripReferenceMarkers {
		text = textclean.StripReferenceMarkers(text)
	}
	return sanitizeLeakedOutput(text, preserveToolMarkup)
}
