package logtail

import "bytes"

// lineWriter is an io.Writer that buffers incoming bytes, splits on '\n',
// and calls emit for each complete line. Lines that grow beyond MaxLineBytes
// without a newline are emitted as truncated and the remainder of that line
// is skipped until the next '\n'.
//
// lineWriter is NOT safe for concurrent use — callers must synchronise
// externally (stdcopy.StdCopy drives it from a single goroutine per stream).
type lineWriter struct {
	containerID string
	stream      string
	emit        func(LogLine)

	buf      []byte // accumulates the current partial line
	overflow bool   // true when the current line already exceeded MaxLineBytes
}

func newLineWriter(containerID, stream string, emit func(LogLine)) *lineWriter {
	return &lineWriter{
		containerID: containerID,
		stream:      stream,
		emit:        emit,
	}
}

// Write receives a byte slice (possibly containing multiple lines or partial
// lines) and emits a LogLine for every complete newline-terminated segment.
func (w *lineWriter) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx == -1 {
			// No newline found — buffer remaining bytes, capping at MaxLineBytes.
			w.appendBuf(p)
			break
		}
		// Found a newline: consume up to and including it.
		segment := p[:idx]
		p = p[idx+1:]

		if w.overflow {
			// Discard the rest of the overflowed line.
			w.overflow = false
			w.buf = w.buf[:0]
			continue
		}

		w.appendBuf(segment)
		if w.overflow {
			// appendBuf already emitted a truncated line; skip the newline and
			// discard any remaining bytes of this line on the next iteration.
			// Reset overflow so the *next* newline terminates the skip.
			w.overflow = false
			w.buf = w.buf[:0]
			continue
		}
		line := string(w.buf)
		w.buf = w.buf[:0]
		w.emit(parseLine(w.containerID, w.stream, line))
	}
	return total, nil
}

// appendBuf appends data to w.buf, enforcing MaxLineBytes. When the cap would
// be exceeded the current buffer is emitted as a truncated line, the overflow
// flag is set, and subsequent bytes until the next '\n' are discarded.
func (w *lineWriter) appendBuf(data []byte) {
	if w.overflow {
		return
	}
	remaining := MaxLineBytes - len(w.buf)
	if len(data) <= remaining {
		w.buf = append(w.buf, data...)
		return
	}
	// Filling the buffer would exceed the cap: emit what we have + truncate marker.
	w.buf = append(w.buf, data[:remaining]...)
	// Re-use parseLine to get the standard structure, then override truncation.
	ll := parseLine(w.containerID, w.stream, string(w.buf))
	// parseLine may itself truncate again; ensure Truncated is set.
	ll.Truncated = true
	// Replace the text cap so the marker isn't doubled.
	if len(ll.Text) > MaxLineBytes-len(truncationMarker) {
		ll.Text = ll.Text[:MaxLineBytes-len(truncationMarker)] + truncationMarker
	}
	w.emit(ll)
	w.buf = w.buf[:0]
	w.overflow = true
}

// flush emits any remaining buffered content as an unterminated (no final
// newline) line. Called when the read stream ends cleanly.
func (w *lineWriter) flush() {
	if len(w.buf) == 0 || w.overflow {
		w.buf = w.buf[:0]
		w.overflow = false
		return
	}
	w.emit(parseLine(w.containerID, w.stream, string(w.buf)))
	w.buf = w.buf[:0]
}
