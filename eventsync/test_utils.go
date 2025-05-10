package eventsync

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// GetTestLogger는 테스트에서 사용할 로거를 생성합니다.
// 환경 변수 LOG_LEVEL을 통해 로그 레벨을 제어할 수 있습니다.
func GetTestLogger() *zap.Logger {
	// 환경 변수에서 로그 레벨 읽기
	logLevelStr := os.Getenv("LOG_LEVEL")
	if logLevelStr == "" {
		// 환경 변수가 설정되지 않은 경우 기본값 사용 (INFO)
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		logger, err := config.Build()
		if err != nil {
			// 로거 초기화 실패 시 기본 로거 사용
			return zap.NewExample()
		}
		return logger
	}

	// 환경 변수에서 로그 레벨 파싱
	var level zapcore.Level
	switch strings.ToUpper(logLevelStr) {
	case "DEBUG":
		level = zapcore.DebugLevel
	case "INFO":
		level = zapcore.InfoLevel
	case "WARN":
		level = zapcore.WarnLevel
	case "ERROR":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	logger, err := config.Build()
	if err != nil {
		// 로거 초기화 실패 시 기본 로거 사용
		return zap.NewExample()
	}
	return logger
}
