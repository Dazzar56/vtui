package textlayout

import (
	"sort"
    "unicode/utf8"

	"github.com/unxed/vtui/piecetable"
	"github.com/mattn/go-runewidth"
)

// LineFragment описывает один визуальный кусок логической строки после свертки.
type LineFragment struct {
	LogicalLineIdx  int // Номер оригинальной строки (до \n)
	ByteOffsetStart int // Смещение начала фрагмента (от начала всего файла/буфера)
	ByteOffsetEnd   int // Смещение конца фрагмента
	VisualWidth     int // Ширина фрагмента в колонках терминала (учитывая CJK)
}

// WrapEngine отвечает за вычисление визуальной разметки текста.
type WrapEngine struct {
	pt         *piecetable.PieceTable
	li         *piecetable.LineIndex
	wrapWidth  int
	wordWrap   bool
	fragmentCache map[int][]LineFragment

	// rowOffsets[i] хранит общее количество визуальных строк во всех
	// логических строках ПЕРЕД строкой i.
	rowOffsets []int
	totalRows  int
	cacheValid bool
}

func NewWrapEngine(pt *piecetable.PieceTable, li *piecetable.LineIndex) *WrapEngine {
	return &WrapEngine{
		pt:            pt,
		li:            li,
		wrapWidth:     80,
		wordWrap:      true,
		fragmentCache: make(map[int][]LineFragment),
	}
}

// SetWidth устанавливает ширину для свертки. При изменении сбрасывает кэш.
func (we *WrapEngine) SetWidth(width int) {
	if width < 1 { width = 1 } // Ширина не может быть меньше 1
	if width != we.wrapWidth {
		we.wrapWidth = width
		we.InvalidateCache()
	}
}

// ToggleWrap включает/выключает перенос по словам.
func (we *WrapEngine) ToggleWrap(wrap bool) {
	if wrap != we.wordWrap {
		we.wordWrap = wrap
		we.InvalidateCache()
	}
}

// InvalidateCache сбрасывает кэш фрагментов.
func (we *WrapEngine) InvalidateCache() {
	we.fragmentCache = make(map[int][]LineFragment)
	we.cacheValid = false
	we.rowOffsets = nil
	we.totalRows = 0
}

// GetFragments возвращает визуальные фрагменты для одной логической строки.
func (we *WrapEngine) GetFragments(logLineIdx int) []LineFragment {
	if frags, ok := we.fragmentCache[logLineIdx]; ok {
		return frags
	}

	if logLineIdx < 0 || logLineIdx >= we.li.LineCount() {
		return nil
	}

	startOffset := we.li.GetLineOffset(logLineIdx)
	endOffset := we.pt.Size()
	if logLineIdx+1 < we.li.LineCount() {
		endOffset = we.li.GetLineOffset(logLineIdx + 1)
	}

	lineData := we.pt.GetRange(startOffset, endOffset-startOffset)
	// Убираем \n или \r\n с конца
	if len(lineData) > 0 && lineData[len(lineData)-1] == '\n' {
		lineData = lineData[:len(lineData)-1]
		if len(lineData) > 0 && lineData[len(lineData)-1] == '\r' {
			lineData = lineData[:len(lineData)-1]
		}
	}

	if !we.wordWrap || we.wrapWidth <= 0 {
		width := runewidth.StringWidth(string(lineData))
		frag := LineFragment{
			LogicalLineIdx:  logLineIdx,
			ByteOffsetStart: startOffset,
			ByteOffsetEnd:   startOffset + len(lineData),
			VisualWidth:     width,
		}
		we.fragmentCache[logLineIdx] = []LineFragment{frag}
		return []LineFragment{frag}
	}

	var fragments []LineFragment
	bytePos := 0
	dataLen := len(lineData)

	for bytePos < dataLen {
		visualWidth := 0
		fragStartByte := bytePos
		lastSpaceEnd := -1
		lastSpaceWidth := 0

		scanPos := bytePos
		for scanPos < dataLen {
			r, size := utf8.DecodeRune(lineData[scanPos:])
			w := runewidth.RuneWidth(r)
			if w < 0 { w = 1 }

			if visualWidth+w > we.wrapWidth {
				if r == ' ' {
					// Пробел не влезает, но мы его забираем в конец этой строки
					scanPos += size
					visualWidth += w
				} else if lastSpaceEnd != -1 {
					// Word Wrap: откатываемся к последнему пробелу
					scanPos = lastSpaceEnd
					visualWidth = lastSpaceWidth
				} else if scanPos == fragStartByte {
					// Даже один символ не влез (CJK в узком окне) - поглощаем его
					scanPos += size
					visualWidth = w
				}
				break
			}

			visualWidth += w
			scanPos += size
			if r == ' ' {
				lastSpaceEnd = scanPos
				lastSpaceWidth = visualWidth
			}
		}

		fragments = append(fragments, LineFragment{
			LogicalLineIdx:  logLineIdx,
			ByteOffsetStart: startOffset + fragStartByte,
			ByteOffsetEnd:   startOffset + scanPos,
			VisualWidth:     visualWidth,
		})
		bytePos = scanPos
	}

	if len(fragments) == 0 {
		fragments = append(fragments, LineFragment{LogicalLineIdx: logLineIdx, ByteOffsetStart: startOffset, ByteOffsetEnd: startOffset})
	}

	we.fragmentCache[logLineIdx] = fragments
	return fragments
}

func (we *WrapEngine) ensureRowCountCache() {
	lineCount := we.li.LineCount()
	if we.cacheValid && len(we.rowOffsets) == lineCount {
		return
	}
	we.rowOffsets = make([]int, lineCount)
	currentOffset := 0
	for i := 0; i < lineCount; i++ {
		we.rowOffsets[i] = currentOffset
		currentOffset += len(we.GetFragments(i))
	}
	we.totalRows = currentOffset
	we.cacheValid = true
}

// GetTotalVisualRows возвращает общее количество визуальных строк в документе.
func (we *WrapEngine) GetTotalVisualRows() int {
	we.ensureRowCountCache()
	return we.totalRows
}

// GetLogLineAtVisualRow переводит абсолютный индекс визуальной строки в индекс
// логической строки и порядковый номер фрагмента внутри неё.
func (we *WrapEngine) GetLogLineAtVisualRow(visualRow int) (logLineIdx int, fragIdx int) {
	we.ensureRowCountCache()
	if visualRow < 0 { return 0, 0 }
	if visualRow >= we.totalRows { return we.li.LineCount() - 1, 0 }

	// Бинарный поиск O(log N)
	logLineIdx = sort.Search(len(we.rowOffsets), func(i int) bool {
		return we.rowOffsets[i] > visualRow
	}) - 1

	if logLineIdx < 0 { logLineIdx = 0 }
	fragIdx = visualRow - we.rowOffsets[logLineIdx]
	return
}

// LogicalToVisual переводит байтовый оффсет в документе в (строка, колонка) на экране.
func (we *WrapEngine) LogicalToVisual(byteOffset int) (visualRow, visualCol int) {
	we.ensureRowCountCache()
	logLineIdx := we.li.GetLineAtOffset(byteOffset)
	fragments := we.GetFragments(logLineIdx)

	// O(1) получение базового ряда из префиксных сумм
	totalRow := we.rowOffsets[logLineIdx]

	for i, frag := range fragments {
		// Используем < для конца фрагмента, чтобы оффсет на границе
		// переходил на следующую строку.
		// Исключение — самый конец последней строки файла.
		isLastFrag := (logLineIdx == we.li.LineCount()-1) && (i == len(fragments)-1)

		if byteOffset >= frag.ByteOffsetStart && (byteOffset < frag.ByteOffsetEnd || (isLastFrag && byteOffset == frag.ByteOffsetEnd)) {
			textBefore := string(we.pt.GetRange(frag.ByteOffsetStart, byteOffset-frag.ByteOffsetStart))
			return totalRow + i, runewidth.StringWidth(textBefore)
		}
	}

	// Если мы в конце фрагмента, но не в конце файла — ищем следующий фрагмент
	return totalRow, 0
}

// VisualToLogical переводит (строка, колонка) на экране в байтовый оффсет документа.
func (we *WrapEngine) VisualToLogical(visualRow, visualCol int) int {
	logLineIdx, fragIdx := we.GetLogLineAtVisualRow(visualRow)
	fragments := we.GetFragments(logLineIdx)
	if fragments == nil {
		return 0
	}
	if fragIdx >= len(fragments) {
		fragIdx = len(fragments) - 1
	}
	frag := fragments[fragIdx]

	if frag.ByteOffsetStart >= frag.ByteOffsetEnd || visualCol <= 0 {
		return frag.ByteOffsetStart
	}

	lineData := we.pt.GetRange(frag.ByteOffsetStart, frag.ByteOffsetEnd-frag.ByteOffsetStart)
	offset := frag.ByteOffsetStart
	currentCol := 0

	for len(lineData) > 0 {
		r, size := utf8.DecodeRune(lineData)
		rw := runewidth.RuneWidth(r)
		if rw <= 0 {
			rw = 1
		}
		if currentCol+rw > visualCol {
			return offset
		}
		currentCol += rw
		offset += size
		lineData = lineData[size:]
	}
	return offset
}
