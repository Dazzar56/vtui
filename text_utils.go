package vtui

import (
	"strings"
	"github.com/mattn/go-runewidth"
)

// WrapText разбивает строку на массив строк, не превышающих maxWidth.
// Учитывает переносы \n и старается разбивать по пробелам.
func WrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	var result []string
	paragraphs := strings.Split(text, "\n")

	for _, p := range paragraphs {
		words := strings.Fields(p)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}

		var currentLine strings.Builder
		currentLineWidth := 0

		for _, word := range words {
			wordWidth := runewidth.StringWidth(word)

			// Если слово само по себе длиннее maxWidth, разбиваем его насильно
			if wordWidth > maxWidth {
				if currentLineWidth > 0 {
					result = append(result, currentLine.String())
					currentLine.Reset()
					currentLineWidth = 0
				}

				runes := []rune(word)
				for len(runes) > 0 {
					chunk := ""
					width := 0
					for i, r := range runes {
						rw := runewidth.RuneWidth(r)
						if width+rw > maxWidth {
							chunk = string(runes[:i])
							runes = runes[i:]
							break
						}
						width += rw
						if i == len(runes)-1 {
							chunk = string(runes)
							runes = nil
						}
					}
					result = append(result, chunk)
				}
				continue
			}

			// Проверяем, влезет ли слово в текущую строку
			spaceWidth := 0
			if currentLineWidth > 0 {
				spaceWidth = 1
			}

			if currentLineWidth+spaceWidth+wordWidth > maxWidth {
				result = append(result, currentLine.String())
				currentLine.Reset()
				currentLine.WriteString(word)
				currentLineWidth = wordWidth
			} else {
				if spaceWidth > 0 {
					currentLine.WriteByte(' ')
				}
				currentLine.WriteString(word)
				currentLineWidth += spaceWidth + wordWidth
			}
		}
		if currentLineWidth > 0 {
			result = append(result, currentLine.String())
		}
	}

	return result
}