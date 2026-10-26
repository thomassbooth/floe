package domain

import "slices"

// Stack holds free-moving windows: where each one is and the order they are
// stacked in, back to front.
type Stack struct {
	order  []WindowID
	frames map[WindowID]Rect
}

// NewStack returns an empty stack.
func NewStack() *Stack {
	return &Stack{frames: make(map[WindowID]Rect)}
}

// Len returns the number of windows.
func (s *Stack) Len() int { return len(s.order) }

// Add puts a window on top of the stack.
func (s *Stack) Add(id WindowID, frame Rect) {
	if _, ok := s.frames[id]; !ok {
		s.order = append(s.order, id)
	}
	s.frames[id] = frame
}

// Remove takes a window out. It reports whether the window was there.
func (s *Stack) Remove(id WindowID) bool {
	if _, ok := s.frames[id]; !ok {
		return false
	}
	delete(s.frames, id)
	s.order = slices.DeleteFunc(s.order, func(o WindowID) bool { return o == id })
	return true
}

// Raise moves a window to the top. It reports whether the order changed.
func (s *Stack) Raise(id WindowID) bool {
	i := slices.Index(s.order, id)
	if i < 0 || i == len(s.order)-1 {
		return false
	}
	s.order = append(slices.Delete(s.order, i, i+1), id)
	return true
}

// Frame returns where a window is.
func (s *Stack) Frame(id WindowID) (Rect, bool) {
	r, ok := s.frames[id]
	return r, ok
}

// SetFrame moves a window. It reports whether the frame changed.
func (s *Stack) SetFrame(id WindowID, r Rect) bool {
	old, ok := s.frames[id]
	if !ok || old == r {
		return false
	}
	s.frames[id] = r
	return true
}

// IDs returns the windows back to front.
func (s *Stack) IDs() []WindowID {
	return slices.Clone(s.order)
}
