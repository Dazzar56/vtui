package vtui

import (
	"fmt"
	"strings"
	"github.com/mattn/go-runewidth"
)

const SemanticSceneVersion = 2

// SemanticSceneAdapter позволяет приложению (например, f4) модифицировать
// сгенерированную сцену перед ее отправкой рендереру.
type SemanticSceneAdapter func(ctx *SemanticContext, baseScene map[string]any) map[string]any

var AppSceneAdapter SemanticSceneAdapter

// SemanticID генерирует уникальный ID для элемента.
func SemanticID(v any) string {
	if v == nil {
		return ""
	}
	if el, ok := v.(UIElement); ok {
		if id := el.GetId(); id != "" {
			return "id:" + id
		}
	}
	return fmt.Sprintf("%T:%p", v, v)
}

// ExportSemanticScene обходит все текущие экраны и фреймы, собирая полное дерево.
func (fm *frameManager) ExportSemanticScene() map[string]any {
	if fm == nil || fm.scr == nil {
		return nil
	}

	ctx := &SemanticContext{
		Width:        fm.scr.width,
		Height:       fm.scr.height,
		ActiveScreen: fm.ActiveIdx,
	}

	fm.SyncCurrentScreen()
	screens := make([]map[string]any, 0, len(fm.Screens))
	for i, screen := range fm.Screens {
		frames := make([]map[string]any, 0, len(screen.Frames))
		for _, frame := range screen.Frames {
			if node := semanticFrame(ctx, frame); node != nil {
				frames = append(frames, node)
			}
		}
		screens = append(screens, map[string]any{
			"index":       i,
			"active":      i == fm.ActiveIdx,
			"title":       screen.GetTitle(),
			"progress":    screen.GetProgress(),
			"attention":   screen.NeedsAttention(),
			"transparent": screen.Transparent,
			"frames":      frames,
		})
	}

	scene := map[string]any{
		"type":         "scene",
		"version":      SemanticSceneVersion,
		"width":        fm.scr.width,
		"height":       fm.scr.height,
		"activeScreen": fm.ActiveIdx,
		"screens":      screens,
	}

	if fm.ActiveIdx >= 0 && fm.ActiveIdx < len(screens) {
		scene["frames"] = screens[fm.ActiveIdx]["frames"]
	}

	if mb := fm.GetActiveMenuBar(); mb != nil {
		scene["menuBar"] = semanticMenuBar(mb)
	}
	if fm.KeyBar != nil {
		scene["keyBar"] = semanticKeyBar(fm.KeyBar)
	}
	if fm.currentToast != nil {
		scene["toast"] = map[string]any{"message": fm.currentToast.Message}
	}
	if len(fm.Screens) > 1 {
		scene["workspaceCount"] = len(fm.Screens)
	}

	if AppSceneAdapter != nil {
		if adapted := AppSceneAdapter(ctx, scene); adapted != nil {
			return adapted
		}
	}
	return scene
}

func semanticFrame(ctx *SemanticContext, frame Frame) map[string]any {
	if frame == nil {
		return nil
	}
	// Если фрейм сам знает, как себя экспортировать (например, PanelsFrame)
	if sp, ok := frame.(SemanticProvider); ok {
		if node := sp.SemanticNode(ctx); node != nil {
			return node
		}
	}

	// Базовый экспорт для стандартных компонентов vtui
	x1, y1, x2, y2 := frame.GetPosition()
	base := map[string]any{
		"id":       SemanticID(frame),
		"title":    strings.TrimSpace(frame.GetTitle()),
		"type":     int(frame.GetType()),
		"x":        x1,
		"y":        y1,
		"w":        x2 - x1 + 1,
		"h":        y2 - y1 + 1,
		"modal":    frame.IsModal(),
		"busy":     frame.IsBusy(),
		"progress": frame.GetProgress(),
		"shadow":   frame.HasShadow(),
	}

	switch f := frame.(type) {
	case *Window:
		base["kind"] = "window"
		if f.Modal {
			base["kind"] = "dialog"
		}
		// Базовый экспорт потомков окна пока опускаем (их будет собирать f4),
		// но можно добавить рекурсивный сбор базовых контролов
		base["showClose"] = f.ShowClose
		base["showZoom"] = f.ShowZoom
		return base
	case *VMenu:
		base["kind"] = "menu"
		base["selected"] = f.SelectPos
		base["top"] = f.TopPos

		items := make([]map[string]any, 0, len(f.Items))
		for i, item := range f.Items {
			clean, hotkey, _ := ParseAmpersandString(item.Text)
			items = append(items, map[string]any{
				"index":     i,
				"text":      clean,
				"rawText":   item.Text,
				"hotkey":    stringOrEmpty(hotkey),
				"shortcut":  item.Shortcut,
				"command":   item.Command,
				"separator": item.Separator,
				"disabled":  false, // FrameManager checks usually happen outside
			})
		}
		base["items"] = items
		return base
	default:
		base["kind"] = "fallback"
		base["fallback"] = true
		base["reason"] = fmt.Sprintf("unsupported frame %T", frame)
		return base
	}
}

func semanticMenuBar(mb *MenuBar) map[string]any {
	if mb == nil {
		return nil
	}
	items := make([]map[string]any, 0, len(mb.Items))
	for i, item := range mb.Items {
		clean, hotkey, _ := ParseAmpersandString(item.Label)
		itemX := mb.GetItemX(i)

		itemW := 0
		if i < len(mb.Items)-1 {
			itemW = mb.GetItemX(i+1) - itemX
		} else {
			itemW = runewidth.StringWidth("  " + clean + "  ")
		}

		subItems := make([]map[string]any, 0, len(item.SubItems))
		for j, sub := range item.SubItems {
			subClean, subHotkey, _ := ParseAmpersandString(sub.Text)
			subItems = append(subItems, map[string]any{
				"index":     j,
				"text":      subClean,
				"rawText":   sub.Text,
				"hotkey":    stringOrEmpty(subHotkey),
				"shortcut":  sub.Shortcut,
				"command":   sub.Command,
				"separator": sub.Separator,
			})
		}

		items = append(items, map[string]any{
			"index":    i,
			"x":        itemX,
			"w":        itemW,
			"text":     clean,
			"rawText":  item.Label,
			"hotkey":   stringOrEmpty(hotkey),
			"command":  item.Command,
			"disabled": false,
			"items":    subItems,
		})
	}
	x1, y1, x2, y2 := mb.GetPosition()
	return map[string]any{
		"id":       SemanticID(mb),
		"kind":     "menuBar",
		"x":        x1,
		"y":        y1,
		"w":        x2 - x1 + 1,
		"h":        y2 - y1 + 1,
		"active":   mb.Active,
		"selected": mb.SelectPos,
		"items":    items,
	}
}

func semanticKeyBar(kb *KeyBar) map[string]any {
	labels := kb.Normal
	modifier := "normal"
	if kb.shiftState {
		labels = kb.Shift
		modifier = "shift"
	} else if kb.ctrlState {
		labels = kb.Ctrl
		modifier = "ctrl"
	} else if kb.altState {
		labels = kb.Alt
		modifier = "alt"
	}
	items := make([]map[string]any, 0, len(labels))
	for i, label := range labels {
		items = append(items, map[string]any{
			"index": i,
			"key":   fmt.Sprintf("F%d", i+1),
			"text":  label,
		})
	}
	x1, y1, x2, y2 := kb.GetPosition()
	return map[string]any{
		"id":       SemanticID(kb),
		"kind":     "keyBar",
		"x":        x1,
		"y":        y1,
		"w":        x2 - x1 + 1,
		"h":        y2 - y1 + 1,
		"visible":  kb.IsVisible(),
		"modifier": modifier,
		"items":    items,
	}
}

func stringOrEmpty(r rune) string {
	if r == 0 {
		return ""
	}
	return string(r)
}