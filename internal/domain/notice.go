package domain

// NoticeKind says what a pane's program did.
type NoticeKind int

const (
	// NoticeTitle reports the pane's new title in Text.
	NoticeTitle NoticeKind = iota
	// NoticeClipboard asks for Text to be copied to the clipboard.
	NoticeClipboard
	// NoticeBell reports that the program rang the bell.
	NoticeBell
)

// PaneNotice is something a pane's program did that the session should
// hear about.
type PaneNotice struct {
	Pane PaneID
	Kind NoticeKind
	Text string
}
