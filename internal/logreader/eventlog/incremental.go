package eventlog

import (
	"bufio"
	"context"
	"github.com/SteelMorgan/1c-log-checker/internal/domain"
	"io"
	"os"
	"regexp"
)

var recordStart = regexp.MustCompile(`^[ \t]*,?[ \t]*\{[0-9]{14},[UCRN],`)

// EOF snapshot может разрезать старую запись. В формате LGP новая запись
// начинается на новой строке с timestamp и статуса транзакции. Остаток старой
// записи отбрасывается; состояние синхронизации сохраняется вместе с offset.
func findRecordStart(ctx context.Context, file *os.File, start int64) (int64, bool, error) {
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return start, false, err
	}
	reader := bufio.NewReaderSize(file, 32768)
	position := start
	for {
		if err := ctx.Err(); err != nil {
			return position, false, err
		}
		prefix, err := reader.Peek(64)
		if recordStart.Match(prefix) {
			return position, true, nil
		}
		if err != nil && err != io.EOF {
			return position, false, err
		}
		lineStart := position
		for {
			chunk, err := reader.ReadSlice('\n')
			position += int64(len(chunk))
			if err == bufio.ErrBufferFull {
				continue
			}
			if err == io.EOF {
				return lineStart, false, nil
			}
			if err != nil {
				return lineStart, false, err
			}
			break
		}
	}
}

// Возвращаемая позиция — граница завершённой записи, а не позиция prefetch
// файлового дескриптора. Незавершённая запись перечитывается после дописывания.
// Этот путь включён только для явно выбранных tail-корней, legacy не меняется.
func (p *LgpParser) ParseIncremental(ctx context.Context, file *os.File, out chan<- *domain.EventLogRecord, start int64, pending bool) (next int64, stillPending bool, err error) {
	if pending {
		next, found, err := findRecordStart(ctx, file, start)
		if err != nil || !found {
			return next, true, err
		}
		start = next
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return start, false, err
	}
	err = p.ParseStream(ctx, file, out, nil, start)
	return p.checkpoint, false, err
}
