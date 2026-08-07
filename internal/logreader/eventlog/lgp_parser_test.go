package eventlog

import (
	"strings"
	"testing"
)

// Формат журнала регистрации 1С не использует бэкслеш как экранирующий символ:
// внутри строки он является обычным символом данных и встречается в конце путей
// (C:\Обмен\), а кавычка экранируется удвоением ("").
//
// Если считать пару \" экранированной кавычкой, закрывающая кавычка строки
// теряется, парсер остаётся в состоянии "внутри строки", перестаёт считать
// фигурные скобки и склеивает все последующие записи в одну.
const lgpWithTrailingBackslash = `1CV8LOG(ver 2.0)
00000000-1111-2222-3333-444444444444

{20260807000000,C,
{2455d34f248f0,a621f1ea},13,62,6,387027,6,I,"Каталог обмена: C:\Обмен\",0,
{"U"},"",2,38,38,713101,0,
{0}
},
{20260807000001,C,
{2455d34f248f1,a621f1eb},13,62,6,387027,6,I,"",0,
{"U"},"",2,38,38,713102,0,
{0}
},
{20260807000002,C,
{2455d34f248f2,a621f1ec},13,62,6,387027,6,I,"обычный комментарий",0,
{"U"},"",2,38,38,713103,0,
{0}
}
`

// Кавычка внутри строки, экранированная удвоением, не должна ломать границы
// записей: пара "" остаётся частью значения.
const lgpWithDoubledQuotes = `1CV8LOG(ver 2.0)
00000000-1111-2222-3333-444444444444

{20260807000000,C,
{2455d34f248f0,a621f1ea},13,62,6,387027,6,I,"{""EntityId"":""A-1"",""Nested"":{""Flag"":true}}",0,
{"U"},"",2,38,38,713101,0,
{0}
},
{20260807000001,C,
{2455d34f248f1,a621f1eb},13,62,6,387027,6,I,"",0,
{"U"},"",2,38,38,713102,0,
{0}
}
`

func TestParseKeepsBoundariesAfterTrailingBackslash(t *testing.T) {
	p := NewLgpParser("cluster", "infobase", "", "", nil)

	records, err := p.Parse(strings.NewReader(lgpWithTrailingBackslash))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d: a backslash before the closing quote swallowed it and merged the records", len(records))
	}

	// Известное ограничение: разбор самого значения с завершающим бэкслешем
	// остаётся неточным — поле вбирает хвост записи. Границы записей при этом
	// восстанавливаются, и остальные записи файла разбираются корректно.
	if records[1].Comment != "" || !strings.Contains(records[2].Comment, "обычный комментарий") {
		t.Errorf("records after the backslash one were not parsed correctly: %q / %q",
			records[1].Comment, records[2].Comment)
	}
}

func TestParseKeepsBoundariesWithDoubledQuotes(t *testing.T) {
	p := NewLgpParser("cluster", "infobase", "", "", nil)

	records, err := p.Parse(strings.NewReader(lgpWithDoubledQuotes))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if !strings.Contains(records[0].Comment, "EntityId") {
		t.Errorf("first record lost its JSON comment: %q", records[0].Comment)
	}
}
