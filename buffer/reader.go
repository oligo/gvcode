package buffer

import (
	"errors"
	"io"
	"unicode/utf8"
)

var _ TextSource = (*PieceTableReader)(nil)

// PieceTableReader implements a [TextSource].
type PieceTableReader struct {
	*PieceTable
	// Index of the slice saves the continuous line number starting from zero.
	// The value contains the rune length of the line.
	lines      []lineInfo
	lastPiece  *piece
	seekCursor int64
}

// ReadAt implements [io.ReaderAt].
func (r *PieceTableReader) ReadAt(p []byte, offset int64) (total int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if offset >= int64(r.seqBytes) {
		return 0, io.EOF
	}

	var expected = len(p)
	var bytes int64
	for n := r.pieces.Head(); n != r.pieces.tail; n = n.next {
		bytes += int64(n.byteLength)

		if bytes > offset {
			fragment := r.getBuf(n.source).getTextByRange(
				n.byteOff+n.byteLength-int(bytes-offset), // calculate the offset in the source buffer.
				int(bytes-offset))

			n := copy(p, fragment)
			p = p[n:]
			total += n
			offset += int64(n)

			if total >= expected {
				break
			}
		}

	}

	if total < expected {
		err = io.EOF
	}

	return
}

// Seek implements [io.Seeker].
func (r *PieceTableReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.seekCursor = offset
	case io.SeekCurrent:
		r.seekCursor += offset
	case io.SeekEnd:
		r.seekCursor = int64(r.seqBytes) + offset
	}
	return r.seekCursor, nil
}

// Read implements [io.Reader].
func (r *PieceTableReader) Read(p []byte) (int, error) {
	n, err := r.ReadAt(p, r.seekCursor)
	r.seekCursor += int64(n)
	return n, err
}

func (r *PieceTableReader) Text(buf []byte) []byte {
	if cap(buf) < int(r.seqBytes) {
		buf = make([]byte, r.seqBytes)
	}
	buf = buf[:r.seqBytes]
	r.Seek(0, io.SeekStart)
	n, _ := io.ReadFull(r, buf)
	buf = buf[:n]
	return buf
}

func (r *PieceTableReader) Lines() int {
	r.lines = r.lines[:0]
	for n := r.PieceTable.pieces.Head(); n != r.PieceTable.pieces.tail; n = n.next {
		pieceText := r.PieceTable.getBuf(n.source).getTextByRange(n.byteOff, n.byteLength)
		lines := r.parseLine(pieceText)
		if len(lines) > 0 {
			if len(r.lines) > 0 {
				lastLine := r.lines[len(r.lines)-1]
				if !lastLine.hasLineBreak {
					// merge with lastLine
					lines[0].length += lastLine.length
					r.lines = r.lines[:len(r.lines)-1]
				}
			}

			r.lines = append(r.lines, lines...)
		}
	}

	return len(r.lines)
}

func (r *PieceTableReader) ReadLine(lineNum int) (line []byte, runeOff int, err error) {
	if lineNum >= len(r.PieceTable.lines) {
		return nil, 0, io.EOF
	}

	lineLen := 0
	for i, lineInfo := range r.PieceTable.lines {
		if i >= lineNum {
			lineLen = lineInfo.length
			break
		}

		runeOff += lineInfo.length
	}

	startBytes := r.RuneOffset(runeOff)
	endBytes := r.RuneOffset(runeOff + lineLen)
	line = make([]byte, endBytes-startBytes)
	r.ReadAt(line, int64(startBytes))
	return
}

// RuneOffset returns the byte offset for the rune at position runeOff.
func (r *PieceTableReader) RuneOffset(runeOff int) int {
	if r.seqLength == 0 {
		return 0
	}

	if runeOff >= r.seqLength {
		return r.seqBytes
	}

	var bytes int
	var runes int

	for n := r.pieces.Head(); n != r.pieces.tail; n = n.next {
		if runes+n.length > runeOff {
			return bytes + r.getBuf(n.source).bytesForRange(n.offset, runeOff-runes)
		}

		bytes += n.byteLength
		runes += n.length

	}

	return bytes
}

// Need optimization
func (r *PieceTableReader) ReadRuneAt(runeOff int64) (rune, error) {
	bytesOff := r.RuneOffset(int(runeOff))

	c, _, err := r.ReadRuneAtBytes(int64(bytesOff))
	return c, err
}

// ReadRuneAt reads the rune starting at the given byte offset, if any.
func (r *PieceTableReader) ReadRuneAtBytes(off int64) (rune, int, error) {
	var buf [utf8.UTFMax]byte
	b := buf[:]
	n, err := r.ReadAt(b, off)
	if errors.Is(err, io.EOF) && n > 0 {
		err = nil
	}
	b = b[:n]
	c, s := utf8.DecodeRune(b)
	return c, s, err
}

// ReadRuneAt reads the run prior to the given byte offset, if any.
func (r *PieceTableReader) ReadRuneBeforeBytes(off int64) (rune, int, error) {
	var buf [utf8.UTFMax]byte
	b := buf[:]
	if off < utf8.UTFMax {
		b = b[:off]
		off = 0
	} else {
		off -= utf8.UTFMax
	}
	n, err := r.ReadAt(b, off)
	b = b[:n]
	c, s := utf8.DecodeLastRune(b)
	return c, s, err
}

func (r *PieceTableReader) parseLine(text []byte) []lineInfo {
	var lines []lineInfo

	n := 0
	for _, c := range string(text) {
		n++
		if c == lineBreak {
			lines = append(lines, lineInfo{length: n, hasLineBreak: true})
			n = 0
		}
	}

	// The remaining bytes that don't end with a line break.
	if n > 0 {
		lines = append(lines, lineInfo{length: n})
	}

	return lines
}

func NewTextSource() *PieceTableReader {
	return &PieceTableReader{
		PieceTable: NewPieceTable([]byte("")),
	}
}
