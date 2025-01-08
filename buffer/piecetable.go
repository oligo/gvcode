package buffer

import "unicode/utf8"

type bufSrc uint8
type action uint8

const (
	original bufSrc = iota
	modify
)

const (
	actionUnknown action = iota
	actionInsert
)

type PieceTable struct {
	originalBuf *textBuffer
	modifyBuf   *textBuffer
	// Length of the text sequence in runes.
	seqLength int
	// bytes size of the text sequence.
	seqBytes int
	seqIndex runeOffIndex

	// undo stack and redo stack
	undoStack *pieceRangeStack
	redoStack *pieceRangeStack
	// piece list
	pieces *pieceList

	// last action and action position in rune offset in the text sequence.
	lastAction       action
	lastActionEndIdx int
	// last inserted piece, for insertion optimization purpose.
	lastInsertPiece *piece
}

func NewPieceTable(text []byte) *PieceTable {
	pt := &PieceTable{
		originalBuf: newTextBuffer(),
		modifyBuf:   newTextBuffer(),
		pieces:      newPieceList(),
		undoStack:   &pieceRangeStack{},
		redoStack:   &pieceRangeStack{},
	}
	pt.init(text)

	return pt
}

// Initialize the piece table with the text by adding the text to the original buffer,
// and create the first piece point to the buffer.
func (pt *PieceTable) init(text []byte) {
	_, _, runeCnt := pt.addToBuffer(original, text)
	if runeCnt <= 0 {
		return
	}

	piece := &piece{
		source:     original,
		offset:     0,
		length:     runeCnt,
		byteOff:    0,
		byteLength: len(text),
	}

	pt.pieces.InsertBeforeTail(piece)
	pt.seqLength = piece.length
	pt.seqBytes = piece.byteLength
	// pt.seqIndex = runeOffIndex{src: pt}
}

func (pt *PieceTable) addToBuffer(source bufSrc, text []byte) (int, int, int) {
	if len(text) <= 0 {
		return 0, 0, 0
	}

	if source == original {
		return 0, 0, pt.originalBuf.set(text)
	}

	return pt.modifyBuf.append(text)
}

func (pt *PieceTable) getBuf(source bufSrc) *textBuffer {
	if source == original {
		return pt.originalBuf
	}

	return pt.modifyBuf
}

func (pt *PieceTable) recordAction(action action, runeIndex int) {
	pt.lastAction = action
	pt.lastActionEndIdx = runeIndex
}

// Insert insert text at the logical position specifed by runeIndex. runeIndex is measured by rune.
// There are 2 scenarios need to be handled:
//  1. Insert in the middle of a piece.
//  2. Insert at the boundary of two pieces.
func (pt *PieceTable) Insert(runeIndex int, text string) bool {
	if runeIndex > pt.seqLength || runeIndex < 0 {
		return false
	}

	pt.redoStack.clear()

	// special-case: inserting at the end of a prior insertion at a piece boundary.
	if pt.tryAppendToLastPiece(runeIndex, text) {
		return true
	}

	oldPiece, inRuneOff := pt.pieces.FindPiece(runeIndex)

	if inRuneOff == 0 {
		pt.insertAtBoundary(runeIndex, text, oldPiece)
	} else {
		pt.insertInMiddle(runeIndex, text, oldPiece, inRuneOff)
	}

	return true
}

// Check if this insert action can be optimized by merging the input with previous one.
// multiple characters input won't be merged.
func (pt *PieceTable) tryAppendToLastPiece(runeIndex int, text string) bool {
	if pt.lastAction != actionInsert ||
		runeIndex != pt.lastActionEndIdx ||
		pt.lastInsertPiece == nil ||
		utf8.RuneCountInString(text) > 1 {
		return false
	}

	_, _, textRunes := pt.addToBuffer(modify, []byte(text))

	pt.lastInsertPiece.length += textRunes
	pt.lastInsertPiece.byteLength += len(text)

	pt.seqLength += textRunes
	pt.seqBytes += len(text)
	pt.recordAction(actionInsert, runeIndex+textRunes)

	return true
}

func (pt *PieceTable) insertAtBoundary(runeIndex int, text string, oldPiece *piece) {
	textRuneOff, textByteOff, textRunes := pt.addToBuffer(modify, []byte(text))

	newPiece := &piece{
		source:     modify,
		offset:     textRuneOff,
		length:     textRunes,
		byteOff:    textByteOff,
		byteLength: len(text),
	}
	pt.lastInsertPiece = newPiece

	// insertion is at the boundary of 2 pieces.
	oldPieces := newUndoPieceRange(pt.seqLength, pt.seqBytes, runeIndex)
	oldPieces.AsBoundary(oldPiece)
	pt.undoStack.push(oldPieces)

	newPieces := newPieceRange()
	newPieces.Append(newPiece)
	// swap link the new piece into the sequence
	oldPieces.Swap(newPieces)
	pt.seqLength += textRunes
	pt.seqBytes += len(text)
	pt.recordAction(actionInsert, runeIndex+textRunes)
}

func (pt *PieceTable) insertInMiddle(runeIndex int, text string, oldPiece *piece, inRuneOff int) {
	textRuneOff, textByteOff, textRunes := pt.addToBuffer(modify, []byte(text))

	newPiece := &piece{
		source:     modify,
		offset:     textRuneOff,
		length:     textRunes,
		byteOff:    textByteOff,
		byteLength: len(text),
	}
	pt.lastInsertPiece = newPiece

	// preserve the old pieces as a pieceRange, and push to the undo stack.
	oldPieces := newUndoPieceRange(pt.seqLength, pt.seqBytes, runeIndex)
	oldPieces.Append(oldPiece)
	pt.undoStack.push(oldPieces)

	// spilt the old piece into 2 new pieces, and insert the newly added text.
	newPieces := newPieceRange()

	// Append the left part of the old piece.
	byteLen := pt.getBuf(oldPiece.source).bytesForRange(oldPiece.offset, inRuneOff)
	newPieces.Append(&piece{
		source:     oldPiece.source,
		offset:     oldPiece.offset,
		length:     inRuneOff,
		byteOff:    oldPiece.byteOff,
		byteLength: byteLen,
	})

	// Then the newly added piece.
	newPieces.Append(newPiece)

	//  And the right part of the old piece.
	byteOff := pt.getBuf(oldPiece.source).RuneOffset(oldPiece.offset + inRuneOff)
	byteLen = pt.getBuf(oldPiece.source).bytesForRange(oldPiece.offset+inRuneOff, oldPiece.length-inRuneOff)
	newPieces.Append(&piece{
		source:     oldPiece.source,
		offset:     oldPiece.offset + inRuneOff,
		length:     oldPiece.length - inRuneOff,
		byteOff:    byteOff,
		byteLength: byteLen,
	})

	oldPieces.Swap(newPieces)
	pt.seqLength += textRunes
	pt.seqBytes += len(text)
	pt.recordAction(actionInsert, runeIndex+textRunes)
}

func (pt *PieceTable) undoRedo(src *pieceRangeStack, dest *pieceRangeStack) bool {
	if src.depth() <= 0 {
		return false
	}

	// remove the next event from the source stack
	rng := src.pop()
	// add event onto the destination stack
	dest.push(rng)

	// do the actual work
	pt.restorePieceRange(rng)
	return true
}

func (pt *PieceTable) restorePieceRange(rng *pieceRange) {
	rng.Restore()
	pt.seqLength = rng.seqLength
	pt.seqBytes = rng.seqBytes
}

func (pt *PieceTable) Erase() {

}

func (pt *PieceTable) Replace() {

}

func (pt *PieceTable) Undo() bool {
	return pt.undoRedo(pt.undoStack, pt.redoStack)
}

func (pt *PieceTable) Redo() bool {
	return pt.undoRedo(pt.redoStack, pt.undoStack)
}

// Size returns the total length of the document data in bytes.
func (pt *PieceTable) Length() int64 {
	return int64(pt.seqLength)
}
