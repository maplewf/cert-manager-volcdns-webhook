package volcengine

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	sdk "github.com/volcengine/volcengine-go-sdk/volcengine"
)

// QueryAll is a generic pagination function
func QueryAll[T any](
	pageSize int,
	query func(int, int) ([]T, int, error),
) ([]T, error) {

	if pageSize <= 0 {
		return nil, fmt.Errorf("pageSize must be greater than 0")
	}
	var all []T
	pageNum := 1
	for {
		data, total, err := query(pageNum, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, data...)
		if pageNum*pageSize >= total {
			break
		}
		pageNum++
	}

	return all, nil
}

type LoggerAdapter struct {
	*logrus.Entry
}

func NewLoggerAdapter(logger *logrus.Entry) *LoggerAdapter {
	return &LoggerAdapter{logger}
}

func (l *LoggerAdapter) Log(args ...interface{}) {
	l.Entry.Log(l.Logger.GetLevel(), args...)
}

func (l *LoggerAdapter) DebugByLevel(level sdk.LogLevelType, args ...interface{}) {
	// Simple implementation
	l.Entry.Debug(args...)
}

func (l *LoggerAdapter) SetDebug(debug *bool) {
	// No-op
}

func (l *LoggerAdapter) SetDebugLogLevel(level *sdk.LogLevelType) {
	// No-op
}

func String(v string) *string {
	return &v
}

func Int32(v int32) *int32 {
	return &v
}

func Int32Value(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func StringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func escapeTXTRecordValue(value string) string {
	// TXT records in Volcengine Public DNS do not support double-quote characters
	// Remove all double quotes; this has no impact on ACME challenge tokens
	v := strings.TrimSpace(value)
	if strings.HasPrefix(v, "\"heritage=") {
		v = strings.Trim(v, "\"")
	}
	return strings.ReplaceAll(v, "\"", "")
}

func unescapeTXTRecordValue(value string) string {
	// Keep it complementary to escapeTXTRecordValue; this only trims quotes and spaces
	return strings.TrimSpace(strings.Trim(value, "\""))
}
