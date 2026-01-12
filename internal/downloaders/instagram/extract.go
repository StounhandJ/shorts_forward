package instagram

import (
	"bytes"
	"encoding/json"
	"html"
	"strings"
)

// VideoVersion описывает интересующие нас поля из JSON
type VideoVersion struct {
	URL string `json:"url"`
	// можно добавить width,height,type если нужно
}

// extractFirstVideoURL ищет "video_versions":[...] в html и возвращает первый url
func extractFirstVideoURL(html string) (string, bool) {
	const key = `"video_versions"`

	i := strings.Index(html, key)
	if i == -1 {
		return "", false
	}

	// найти '[' после ключа
	brStart := strings.Index(html[i:], "[")
	if brStart == -1 {
		return "", false
	}
	// brStart индекс относительно html[i:], переводим в общий индекс
	pos := i + brStart

	// пройдём по строке и найдём соответствующую закрывающую ']' (учитывая строки и escape)
	depth := 0
	inString := false
	escaped := false

	end := -1
	for j := pos; j < len(html); j++ {
		c := html[j]

		if escaped {
			escaped = false

			continue
		}

		if c == '\\' {
			// начало escape внутри строки
			escaped = true

			continue
		}

		if c == '"' {
			inString = !inString

			continue
		}

		if inString {
			continue
		}

		if c == '[' {
			depth++

			continue
		}

		if c == ']' {
			depth--
			if depth == 0 {
				end = j

				break
			}
		}
	}

	if end == -1 {
		return "", false
	}

	arrJSON := html[pos : end+1]

	// Разберём JSON в структуру
	var versions []VideoVersion
	if err := json.Unmarshal([]byte(arrJSON), &versions); err != nil {
		// иногда в HTML массив может быть предшествован non-JSON (например single quotes),
		// или внутри содержатся символы, мешающие парсингу - возвращаем ошибку с контекстом
		return "", false
	}

	// Найдём первый непустой url
	for _, v := range versions {
		if strings.TrimSpace(v.URL) != "" {
			// json.Unmarshal уже расшифровал \/ в /
			return v.URL, true
		}
	}

	return "", false
}

func findMetaContent(htmlBytes []byte, prop string) (string, bool) {
	propLower := strings.ToLower(prop)

	// проход по всем вхождениям '<'
	for idx := 0; idx < len(htmlBytes); {
		// найти '<'
		lt := bytes.IndexByte(htmlBytes[idx:], '<')
		if lt == -1 {
			break
		}

		lt += idx

		// простая проверка - следующий фрагмент должен быть meta (как минимум 4 буквы)
		// проверка case-insensitive
		if lt+5 <= len(htmlBytes) && bytes.EqualFold(htmlBytes[lt+1:lt+5], []byte("meta")) {
			// найти конец тега '>'
			rt := bytes.IndexByte(htmlBytes[lt:], '>')
			if rt == -1 {
				break
			}

			rt += lt
			seg := htmlBytes[lt : rt+1]    // сегмент с тегом <meta ...>
			segLower := bytes.ToLower(seg) // небольшой allocation: тег обычно короткий
			propKey := []byte("property=")

			// ищем все property= в сегменте
			searchFrom := 0
			for {
				pIdx := bytes.Index(segLower[searchFrom:], propKey)
				if pIdx == -1 {
					break
				}
				// реальный индекс в seg
				pIdx += searchFrom
				// позиция после 'property='
				pos := pIdx + len(propKey)
				if pos >= len(seg) {
					break
				}
				// определить кавычки (может быть " или ')
				quote := seg[pos]
				if quote != '"' && quote != '\'' {
					// может быть без кавычек - попытаемся прочитать до пробела (реже встречается)
					// читаем until space or >
					end := pos
					for end < len(seg) && seg[end] != ' ' && seg[end] != '\t' && seg[end] != '>' {
						end++
					}

					propVal := string(bytes.TrimSpace(seg[pos:end]))
					if strings.EqualFold(propVal, propLower) {
						// нашли нужный property - теперь получаем content
						if v, ok := extractContentFromMeta(seg, segLower); ok {
							return v, true
						}
					}

					searchFrom = pIdx + 1

					continue
				}
				// найти закрывающую кавычку
				// учтём экранирование внутри (редко в property, но на всякий случай)
				qend := pos + 1
				for qend < len(seg) {
					if seg[qend] == '\\' {
						qend += 2

						continue
					}

					if seg[qend] == quote {
						break
					}

					qend++
				}

				if qend >= len(seg) {
					break
				}

				propValBytes := seg[pos+1 : qend]

				propVal := strings.ToLower(string(propValBytes))
				if propVal == propLower {
					// нашли нужный property - теперь получаем content
					if v, ok := extractContentFromMeta(seg, segLower); ok {
						return v, true
					}
				}

				searchFrom = pIdx + 1
			}
			// продвигаем индекс за текущий тег
			idx = rt + 1

			continue
		}
		// иначе сдвигаемся за '<'
		idx = lt + 1
	}

	return "", false
}

// extractContentFromMeta вытаскивает значение content="..." (или content='...') из сегмента meta.
// seg - оригинальный сегмент, segLower - сегмент в нижнем регистре (для поиска ключей).
func extractContentFromMeta(seg, segLower []byte) (string, bool) {
	contentKey := []byte("content=")

	cIdx := bytes.Index(segLower, contentKey)
	if cIdx == -1 {
		return "", false
	}

	pos := cIdx + len(contentKey)
	if pos >= len(seg) {
		return "", false
	}

	quote := seg[pos]
	if quote == '"' || quote == '\'' {
		// найти конец
		end := pos + 1
		for end < len(seg) {
			if seg[end] == '\\' {
				end += 2

				continue
			}

			if seg[end] == quote {
				break
			}

			end++
		}

		if end >= len(seg) {
			return "", false
		}

		val := seg[pos+1 : end]
		// unescape HTML entities (например &quot; &#x... )
		return html.UnescapeString(string(val)), true
	}
	// без кавычек: читаем до пробела или '>'
	end := pos
	for end < len(seg) && seg[end] != ' ' && seg[end] != '\t' && seg[end] != '>' {
		end++
	}

	val := seg[pos:end]

	return html.UnescapeString(string(val)), true
}
