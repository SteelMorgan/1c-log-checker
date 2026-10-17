package writer

import (
	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"reflect"
)

const maxBatchBytes = 16 * 1024 * 1024

// Оценка включает строки и свойства: число записей само по себе не ограничивает
// батч с большими комментариями. Запас учитывает служебные структуры и копии.
func recordMemoryBytes(record any) int {
	v := reflect.ValueOf(record).Elem()
	size := int(v.Type().Size())
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.String:
			size += field.Len()
		case reflect.Map:
			it := field.MapRange()
			for it.Next() {
				size += it.Key().Len() + it.Value().Len() + 64
			}
		}
	}
	if event, ok := record.(*domain.EventLogRecord); ok && event.SourceBytes > size {
		size = event.SourceBytes
	}
	return size * 2
}
