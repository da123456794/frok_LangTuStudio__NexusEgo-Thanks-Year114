package string_reader

import "fmt"

func (s *StringReader) String() string {
	if s.ctx == nil {
		return ""
	}
	return *s.ctx
}

func (s *StringReader) Pointer() int {
	return s.ptr
}

func (s *StringReader) Reset(str *string) {
	s.ctx = str
	s.ptr = 0
}

func (s *StringReader) SetPtr(new int) {
	if s.ctx == nil || new > len(*s.ctx) {
		switch {
		case s.ctx == nil:
			panic(fmt.Sprintf("SetPtr: Failed to set pointer to %d because c.ctx is nil", new))
		case new > len(*s.ctx):
			panic(fmt.Sprintf("SetPtr: Failed to set pointer to %d because of EOF error", new))
		}
	}
	s.ptr = new
}

func (s *StringReader) Sentence(length int) string {
	if length < 0 {
		panic(fmt.Sprintf("Sentence: The length provided is less than 0; length = %d", length))
	}
	if s.ctx == nil || s.ptr > len(*s.ctx) {
		panic("Sentence: Pointer was broken")
	}
	if l := len(*s.ctx); s.ptr+length > l {
		res := (*s.ctx)[s.ptr:]
		s.ptr = l
		return res
	}
	str := (*s.ctx)[s.ptr : s.ptr+length]
	s.ptr += length
	return str
}

func (s *StringReader) Next(avoidEOF bool) string {
	if str := s.Sentence(1); !avoidEOF && len(str) == 0 {
		panic("Next: EOF")
	} else {
		return str
	}
}

func (s *StringReader) CutSentence(startPoint int) string {
	length := s.ptr - startPoint
	s.SetPtr(startPoint)
	return s.Sentence(length)
}
