package vtui

import "github.com/unxed/vtinput"

// ScreenObject — это базовый класс для всех видимых элементов интерфейса,
// аналог ScreenObject из scrobj.hpp.
type ScreenObject struct {
	X1, Y1, X2, Y2 int
	owner          *ScreenObject
	saveScr        *SaveScreen
	visible        bool
	lockCount      int
}

// SetPosition устанавливает координаты объекта.
// Важно: это не вызывает перерисовку.
func (so *ScreenObject) SetPosition(x1, y1, x2, y2 int) {
	// При изменении позиции сохраненный фон становится невалидным.
	so.saveScr = nil
	so.X1, so.Y1, so.X2, so.Y2 = x1, y1, x2, y2
}

// GetPosition возвращает текущие координаты объекта.
func (so *ScreenObject) GetPosition() (int, int, int, int) {
	return so.X1, so.Y1, so.X2, so.Y2
}

// Show делает объект видимым. Перед отрисовкой он сохраняет область экрана под собой.
func (so *ScreenObject) Show(scr *ScreenBuf) {
	if so.IsLocked() || so.visible {
		return
	}
	so.saveScr = NewSaveScreen(scr, so.X1, so.Y1, so.X2, so.Y2)
	so.visible = true
	// В дочерних классах здесь будет вызов DisplayObject()
}

// Hide скрывает объект и восстанавливает сохраненную под ним область экрана.
func (so *ScreenObject) Hide(scr *ScreenBuf) {
	if !so.visible {
		return
	}
	if so.saveScr != nil {
		so.saveScr.Restore(scr)
		so.saveScr = nil
	}
	so.visible = false
}

// IsVisible возвращает true, если объект видим.
func (so *ScreenObject) IsVisible() bool {
	return so.visible
}

// Lock увеличивает счетчик блокировок. Заблокированный объект не перерисовывается.
func (so *ScreenObject) Lock() {
	so.lockCount++
}

// Unlock уменьшает счетчик блокировок.
func (so *ScreenObject) Unlock() {
	if so.lockCount > 0 {
		so.lockCount--
	}
}

// IsLocked возвращает true, если объект или его владелец заблокирован.
func (so *ScreenObject) IsLocked() bool {
	if so.lockCount > 0 {
		return true
	}
	if so.owner != nil {
		return so.owner.IsLocked()
	}
	return false
}

// ProcessKey (заглушка) будет переопределяться в дочерних классах.
func (so *ScreenObject) ProcessKey(key *vtinput.InputEvent) bool {
	return false
}

// ProcessMouse (заглушка) будет переопределяться в дочерних классах.
func (so *ScreenObject) ProcessMouse(mouse *vtinput.InputEvent) bool {
	return false
}

// ResizeConsole (заглушка) будет переопределяться для реакции на изменение размера.
func (so *ScreenObject) ResizeConsole() {
	// Пустая реализация по умолчанию.
}