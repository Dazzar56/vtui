//go:build linux

package vtui

import (
	"io"
	"testing"
	"time"

	"github.com/unxed/vtinput"
)

func TestWaylandHost_KeyRepeat(t *testing.T) {
	pr, _ := io.Pipe()
	reader := vtinput.NewReader(pr, true)
	defer reader.Close()

	host := &WaylandHost{reader: reader}

	// 1. Запускаем повтор для клавиши 'a' (VK_A)
	host.startRepeat(vtinput.VK_A, 'a', 0)

	// Ждем первое повторяющееся событие (задержка 400мс)
	select {
	case ev := <-reader.EventChan:
		if ev.VirtualKeyCode != vtinput.VK_A || ev.Char != 'a' || !ev.KeyDown {
			t.Errorf("Unexpected event during key repeat: %+v", ev)
		}
	case <-time.After(600 * time.Millisecond):
		t.Fatal("Key repeat event did not arrive in time")
	}

	// 2. Останавливаем повтор
	host.stopRepeat()

	// Очищаем оставшиеся события из буфера канала
	time.Sleep(100 * time.Millisecond)
	for len(reader.EventChan) > 0 {
		<-reader.EventChan
	}

	// 3. Убеждаемся, что новые события больше не поступают
	select {
	case ev := <-reader.EventChan:
		t.Errorf("Received unexpected event after stopRepeat: %+v", ev)
	case <-time.After(100 * time.Millisecond):
		// Успешно: тишина в канале
	}
}