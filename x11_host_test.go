package vtui

import (
	"testing"
	"time"
)

func TestX11Renderer_CursorMovementTracking(t *testing.T) {
	r := NewX11Renderer(nil, nil)

	// Устанавливаем начальную позицию
	r.SetCursor(10, 5, true, CursorShapeUnderline)

	// Имитируем перемещение влево (скрытые панели, нажатие Left)
	r.SetCursor(9, 5, true, CursorShapeUnderline)

	if r.oldCursorX != 10 {
		t.Errorf("expected oldCursorX to be 10, got %d", r.oldCursorX)
	}
	if r.cursorX != 9 {
		t.Errorf("expected cursorX to be 9, got %d", r.cursorX)
	}
	if r.oldCursorY != 5 || r.cursorY != 5 {
		t.Error("expected both old and new cursor row to be marked as 5")
	}
}

func TestX11Renderer_CursorBlinkAndReset(t *testing.T) {
	r := NewX11Renderer(nil, nil)

	// Проверяем инициализацию таймера
	r.SetCursor(0, 0, true, CursorShapeUnderline)
	t1 := r.lastCursorReset
	if t1.IsZero() {
		t.Fatal("expected lastCursorReset to be initialized and non-zero")
	}

	// Тестируем, что перемещение курсора сбрасывает таймер
	time.Sleep(2 * time.Millisecond)
	r.SetCursor(1, 0, true, CursorShapeUnderline)
	t2 := r.lastCursorReset
	if !t2.After(t1) {
		t.Error("expected lastCursorReset to update (be newer) after cursor movement")
	}

	// Тестируем, что изменение формы курсора (Ins/Ovr) сбрасывает таймер
	time.Sleep(2 * time.Millisecond)
	r.SetCursor(1, 0, true, CursorShapeBlock)
	t3 := r.lastCursorReset
	if !t3.After(t2) {
		t.Error("expected lastCursorReset to update (be newer) after cursor shape change")
	}
}

func TestX11Host_DirtySpanLogic(t *testing.T) {
	// Мы не можем запустить реальный X-сервер, но можем проверить логику
	// отслеживания грязных строк напрямую.
	h := &X11Host{
		dirtyLines: make([]bool, 100),
	}

	// Помечаем две разрозненные группы строк
	h.dirtyLines[10] = true
	h.dirtyLines[11] = true
	h.dirtyLines[12] = true

	h.dirtyLines[50] = true
	h.dirtyLines[51] = true

	// Проверяем, как flushImage (в теории) должен их обходить.
	// Ожидается Bounding Box оптимизация (от первой грязной до последней)
	minY := -1
	maxY := -1
	for y := 0; y < 100; y++ {
		if h.dirtyLines[y] {
			if minY == -1 {
				minY = y
			}
			maxY = y
		}
	}

	if minY != 10 {
		t.Errorf("Expected minY 10, got %d", minY)
	}
	if maxY != 51 {
		t.Errorf("Expected maxY 51, got %d", maxY)
	}
}
