package vtui

import "github.com/unxed/vtinput"
import "sync"
import "fmt"

// x11KeysymToVK мапит стандартные X11 Keysyms в Windows Virtual Key Codes.
var x11KeysymToVK = map[uint32]uint16{
	0xff08: vtinput.VK_BACK,
	0xff09: vtinput.VK_TAB,
	0xff0d: vtinput.VK_RETURN,
	0xff1b: vtinput.VK_ESCAPE,
	0xff50: vtinput.VK_HOME,
	0xff51: vtinput.VK_LEFT,
	0xff52: vtinput.VK_UP,
	0xff53: vtinput.VK_RIGHT,
	0xff54: vtinput.VK_DOWN,
	0xff55: vtinput.VK_PRIOR, // PgUp
	0xff56: vtinput.VK_NEXT,  // PgDn
	0xff57: vtinput.VK_END,
	0xff63: vtinput.VK_INSERT,
	0xffff: vtinput.VK_DELETE,
	0xffbe: vtinput.VK_F1,
	0xffbf: vtinput.VK_F2,
	0xffc0: vtinput.VK_F3,
	0xffc1: vtinput.VK_F4,
	0xffc2: vtinput.VK_F5,
	0xffc3: vtinput.VK_F6,
	0xffc4: vtinput.VK_F7,
	0xffc5: vtinput.VK_F8,
	0xffc6: vtinput.VK_F9,
	0xffc7: vtinput.VK_F10,
	0xffc8: vtinput.VK_F11,
	0xffc9: vtinput.VK_F12,
	0xffeb: vtinput.VK_LWIN,     // Left Super/Win
	0xffec: vtinput.VK_RWIN,     // Right Super/Win
	0xff67: vtinput.VK_APPS,     // Menu key
	0xffe1: vtinput.VK_LSHIFT,   // Left Shift
	0xffe2: vtinput.VK_RSHIFT,   // Right Shift
	0xffe3: vtinput.VK_LCONTROL, // Left Ctrl
	0xffe4: vtinput.VK_RCONTROL, // Right Ctrl
	0xffe5: vtinput.VK_CAPITAL,  // Caps Lock
	0xffe9: vtinput.VK_LMENU,    // Left Alt
	0xffea: vtinput.VK_RMENU,    // Right Alt
	0xff7f: vtinput.VK_NUMLOCK,  // Num Lock
	0xff14: vtinput.VK_SCROLL,   // Scroll Lock
	0x0020: vtinput.VK_SPACE,
	0x002b: vtinput.VK_OEM_PLUS,   // +
	0x003d: vtinput.VK_OEM_PLUS,   // =
	0x002d: vtinput.VK_OEM_MINUS,  // -
	0x005f: vtinput.VK_OEM_MINUS,  // _
	0x005b: vtinput.VK_OEM_4,      // [
	0x007b: vtinput.VK_OEM_4,      // {
	0x005d: vtinput.VK_OEM_6,      // ]
	0x007d: vtinput.VK_OEM_6,      // }
	0x003b: vtinput.VK_OEM_1,      // ;
	0x003a: vtinput.VK_OEM_1,      // :
	0x0027: vtinput.VK_OEM_7,      // '
	0x0022: vtinput.VK_OEM_7,      // "
	0x002c: vtinput.VK_OEM_COMMA,  // ,
	0x003c: vtinput.VK_OEM_COMMA,  // <
	0x002e: vtinput.VK_OEM_PERIOD, // .
	0x003e: vtinput.VK_OEM_PERIOD, // >
	0x002f: vtinput.VK_OEM_2,      // /
	0x003f: vtinput.VK_OEM_2,      // ?
	0x005c: vtinput.VK_OEM_5,      // \
	0x007c: vtinput.VK_OEM_5,      // |
	0x0060: vtinput.VK_OEM_3,      // `
	0x007e: vtinput.VK_OEM_3,      // ~

	// Numpad (Keysyms range 0xff80 - 0xffaf)
	0xff8d: vtinput.VK_RETURN,   // KP_Enter
	0xffaa: vtinput.VK_MULTIPLY, // KP_Multiply
	0xffab: vtinput.VK_ADD,      // KP_Add
	0xffad: vtinput.VK_SUBTRACT, // KP_Subtract
	0xffae: vtinput.VK_DECIMAL,  // KP_Decimal
	0xffaf: vtinput.VK_DIVIDE,   // KP_Divide

	// Numpad Digits (when NumLock is ON)
	0xffb0: vtinput.VK_NUMPAD0,
	0xffb1: vtinput.VK_NUMPAD1,
	0xffb2: vtinput.VK_NUMPAD2,
	0xffb3: vtinput.VK_NUMPAD3,
	0xffb4: vtinput.VK_NUMPAD4,
	0xffb5: vtinput.VK_NUMPAD5,
	0xffb6: vtinput.VK_NUMPAD6,
	0xffb7: vtinput.VK_NUMPAD7,
	0xffb8: vtinput.VK_NUMPAD8,
	0xffb9: vtinput.VK_NUMPAD9,

	// Numpad Navigation (when NumLock is OFF)
	0xff95: vtinput.VK_HOME,
	0xff96: vtinput.VK_LEFT,
	0xff97: vtinput.VK_UP,
	0xff98: vtinput.VK_RIGHT,
	0xff99: vtinput.VK_DOWN,
	0xff9a: vtinput.VK_PRIOR,  // PgUp
	0xff9b: vtinput.VK_NEXT,   // PgDn
	0xff9c: vtinput.VK_END,
	0xff9d: vtinput.VK_CLEAR, // Center 5
	0xff9e: vtinput.VK_INSERT,
	0xff9f: vtinput.VK_DELETE,
}

// x11KeysymToRune содержит сопоставление кейсимов и символов Unicode.
// Карта инициализируется здесь и наполняется автоматически в x11_keysym_map_generated.go
var x11KeysymToRune = make(map[uint32]rune)
var keysymMapReported sync.Once

func keysymToRune(keysym uint32) rune {
	res := rune(0)
	defer func() {
		if keysym > 0x7f {
			charRepr := fmt.Sprintf("'%c'", res)
			if res == 0 {
				charRepr = "NULL"
			}
			DebugLog("X11_KEYS_TRACE: keysymToRune(0x%X) -> %s (%d)", keysym, charRepr, res)
		}
	}()
	// 1. Прямое соответствие Unicode (0x0100XXXX)
	if keysym >= 0x01000000 && keysym <= 0x01ffffff {
		return rune(keysym & 0xffffff)
	}

	// 2. Латиница и ASCII (0x0020 - 0x007e)
	if keysym >= 0x0020 && keysym <= 0x007e {
		res = rune(keysym)
		return res
	}

	// 3. Таблица из хедеров (генерируемая)
	if r, ok := x11KeysymToRune[keysym]; ok {
		res = r
		return res
	}

	return 0
}

func keysymToVK(keysym uint32) uint16 {
	// 1. Прямой маппинг спецклавиш
	if vk, ok := x11KeysymToVK[keysym]; ok {
		return vk
	}
	// 2. Цифры
	if keysym >= 0x0030 && keysym <= 0x0039 {
		return uint16(keysym)
	}
	// 3. Латиница (в Keysym латиница совпадает с ASCII)
	if keysym >= 0x0061 && keysym <= 0x007a { // a-z
		return uint16(keysym - 0x20) // to A-Z
	}
	if keysym >= 0x0041 && keysym <= 0x005a { // A-Z
		return uint16(keysym)
	}
	return 0
}