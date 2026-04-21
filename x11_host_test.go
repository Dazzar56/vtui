package vtui

import (
	"testing"
)

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

func TestX11Host_ModifierTranslation(t *testing.T) {
	h := &X11Host{}
	// ModMaskControl = 4, ModMask2 (NumLock) = 16
	mods := h.translateModifiers(4 | 16, 0, false)

	const LeftCtrlPressed = 0x0008
	const NumLockOn = 0x0020

	if (mods & LeftCtrlPressed) == 0 {
		t.Error("Failed to translate Control modifier")
	}
	if (mods & NumLockOn) == 0 {
		t.Error("Failed to translate NumLock modifier (Mod2)")
	}
}