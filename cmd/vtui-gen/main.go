package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PropDef struct {
	Type    string `json:"type"`
	Default any    `json:"default"`
	Summary string `json:"summary"`
}

type SizeSpecDef struct {
	W int `json:"w"`
	H int `json:"h"`
}

type SizePolicyDef struct {
	H string `json:"h"`
	V string `json:"v"`
}

type WidgetDef struct {
	Extends     string             `json:"extends"`
	Summary     string             `json:"summary"`
	Properties  map[string]PropDef `json:"properties"`
	Signals     []string           `json:"signals"`
	Localizable []string           `json:"localizable"`
	SizeHint    *SizeSpecDef       `json:"sizeHint"`
	MinSize     *SizeSpecDef       `json:"minSize"`
	SizePolicy  *SizePolicyDef     `json:"sizePolicy"`
}

type LayoutDef struct {
	Summary    string             `json:"summary"`
	Properties map[string]PropDef `json:"properties"`
}

type Vocabulary struct {
	Version      int                  `json:"version"`
	Widgets      map[string]WidgetDef `json:"widgets"`
	Layouts      map[string]LayoutDef `json:"layouts"`
	Commands     map[string]int       `json:"commands"`
	PaletteRoles []string             `json:"paletteRoles"`
}

func LoadVocabulary(path string) (*Vocabulary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v Vocabulary
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func GenerateWidgetsMarkdown(v *Vocabulary) string {
	var b strings.Builder
	b.WriteString("# Справочник виджетов и свойств vtui\n\n")
	b.WriteString("Этот файл сгенерирован автоматически из `vocabulary.json` с помощью `cmd/vtui-gen`. **Не редактируйте вручную.**\n\n")

	b.WriteString("## Виджеты\n\n")

	names := make([]string, 0, len(v.Widgets))
	for name := range v.Widgets {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		w := v.Widgets[name]
		fmt.Fprintf(&b, "### `%s`\n\n", name)
		fmt.Fprintf(&b, "%s\n\n", w.Summary)

		if w.Extends != "" {
			fmt.Fprintf(&b, "*Наследует:* `%s`\n\n", w.Extends)
		}

		if w.SizeHint != nil || w.MinSize != nil || w.SizePolicy != nil {
			b.WriteString("**Геометрия по умолчанию:**\n")
			if w.SizeHint != nil {
				fmt.Fprintf(&b, "- `sizeHint`: %d × %d ячеек\n", w.SizeHint.W, w.SizeHint.H)
			}
			if w.MinSize != nil {
				fmt.Fprintf(&b, "- `minSize`: %d × %d ячеек\n", w.MinSize.W, w.MinSize.H)
			}
			if w.SizePolicy != nil {
				fmt.Fprintf(&b, "- `sizePolicy`: h=`%s`, v=`%s`\n", w.SizePolicy.H, w.SizePolicy.V)
			}
			b.WriteString("\n")
		}

		if len(w.Properties) > 0 {
			b.WriteString("**Свойства:**\n\n")
			b.WriteString("| Свойство | Тип | По умолчанию | Описание |\n")
			b.WriteString("|---|---|---|---|\n")

			pNames := make([]string, 0, len(w.Properties))
			for pn := range w.Properties {
				pNames = append(pNames, pn)
			}
			sort.Strings(pNames)

			for _, pn := range pNames {
				prop := w.Properties[pn]
				defStr := fmt.Sprintf("%v", prop.Default)
				if prop.Type == "string" {
					defStr = fmt.Sprintf("%q", prop.Default)
				}
				fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", pn, prop.Type, defStr, prop.Summary)
			}
			b.WriteString("\n")
		}

		if len(w.Signals) > 0 {
			b.WriteString("**Сигналы:** ")
			sigList := make([]string, len(w.Signals))
			for i, s := range w.Signals {
				sigList[i] = fmt.Sprintf("`%s`", s)
			}
			b.WriteString(strings.Join(sigList, ", "))
			b.WriteString("\n\n")
		}

		if len(w.Localizable) > 0 {
			b.WriteString("**Локализуемые свойства:** ")
			locList := make([]string, len(w.Localizable))
			for i, l := range w.Localizable {
				locList[i] = fmt.Sprintf("`%s`", l)
			}
			b.WriteString(strings.Join(locList, ", "))
			b.WriteString("\n\n")
		}

		b.WriteString("---\n\n")
	}

	b.WriteString("## Контейнеры раскладки\n\n")
	lNames := make([]string, 0, len(v.Layouts))
	for name := range v.Layouts {
		lNames = append(lNames, name)
	}
	sort.Strings(lNames)

	for _, name := range lNames {
		l := v.Layouts[name]
		fmt.Fprintf(&b, "### `%s`\n\n", name)
		fmt.Fprintf(&b, "%s\n\n", l.Summary)
		if len(l.Properties) > 0 {
			b.WriteString("| Свойство | Тип | По умолчанию | Описание |\n")
			b.WriteString("|---|---|---|---|\n")
			lpNames := make([]string, 0, len(l.Properties))
			for lpn := range l.Properties {
				lpNames = append(lpNames, lpn)
			}
			sort.Strings(lpNames)
			for _, lpn := range lpNames {
				prop := l.Properties[lpn]
				defStr := fmt.Sprintf("%v", prop.Default)
				fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s |\n", lpn, prop.Type, defStr, prop.Summary)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func main() {
	vocabPath := "vocabulary.json"
	if len(os.Args) > 1 {
		vocabPath = os.Args[1]
	}

	v, err := LoadVocabulary(vocabPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading vocabulary: %v\n", err)
		os.Exit(1)
	}

	doc := GenerateWidgetsMarkdown(v)
	docsDir := "docs"
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating docs dir: %v\n", err)
		os.Exit(1)
	}

	outPath := filepath.Join(docsDir, "widgets.md")
	if err := os.WriteFile(outPath, []byte(doc), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outPath, err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s successfully.\n", outPath)
}
