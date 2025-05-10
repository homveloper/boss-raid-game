package testutil

import (
	"flag"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// DefaultLogLevel은 기본 로그 레벨입니다.
	DefaultLogLevel = zapcore.InfoLevel

	// GlobalLogger는 전역 로거 인스턴스입니다.
	GlobalLogger *zap.Logger

	// logLevel은 명령줄 플래그로 지정할 수 있는 로그 레벨입니다.
	logLevel string

	// once는 로거 초기화가 한 번만 수행되도록 보장합니다.
	once sync.Once
)

func init() {
	// 명령줄 플래그 정의
	flag.StringVar(&logLevel, "loglevel", "", "로그 레벨 설정 (debug, info, warn, error)")

	// 환경 변수에서 로그 레벨 읽기
	envLogLevel := os.Getenv("LOG_LEVEL")
	if envLogLevel != "" && logLevel == "" {
		// 환경 변수가 설정되어 있고 플래그가 설정되지 않은 경우
		logLevel = envLogLevel
	}

	// 기본 로그 레벨로 초기화
	// 실제 로거 초기화는 NewLogger() 호출 시 수행됨
	if logLevel == "" {
		initLogger(DefaultLogLevel)
	} else {
		// 로그 레벨 파싱
		var level zapcore.Level
		switch strings.ToUpper(logLevel) {
		case "DEBUG":
			level = zapcore.DebugLevel
		case "INFO":
			level = zapcore.InfoLevel
		case "WARN":
			level = zapcore.WarnLevel
		case "ERROR":
			level = zapcore.ErrorLevel
		default:
			level = DefaultLogLevel
		}
		initLogger(level)
	}
}

// initLogger는 지정된 로그 레벨로 로거를 초기화합니다.
func initLogger(level zapcore.Level) {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	logger, err := config.Build()
	if err != nil {
		// 로거 초기화 실패 시 기본 로거 사용
		GlobalLogger = zap.NewExample()
		GlobalLogger.Error("Failed to initialize logger", zap.Error(err))
		return
	}
	GlobalLogger = logger
}

// NewLogger는 테스트에서 사용할 로거를 생성합니다.
// 이 함수는 전역 로그 레벨 설정을 따르는 새 로거 인스턴스를 반환합니다.
// 로거가 아직 초기화되지 않은 경우 초기화합니다.
func NewLogger() *zap.Logger {
	// 로거가 초기화되지 않은 경우
	if GlobalLogger == nil {
		// 로그 레벨 결정 (우선순위: 명령줄 플래그 > 환경 변수 > 기본값)
		var logLevelStr string
		if logLevel != "" {
			// 명령줄 플래그에서 로그 레벨 읽기
			logLevelStr = logLevel
		} else {
			// 환경 변수에서 로그 레벨 읽기
			logLevelStr = os.Getenv("LOG_LEVEL")
		}

		if logLevelStr == "" {
			// 로그 레벨이 지정되지 않은 경우 기본값 사용
			initLogger(DefaultLogLevel)
		} else {
			// 로그 레벨 파싱
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
				level = DefaultLogLevel
			}
			initLogger(level)
		}
	}

	return GlobalLogger.With(zap.String("context", "test"))
}

// SetLogLevel은 전역 로그 레벨을 동적으로 변경합니다.
func SetLogLevel(level zapcore.Level) {
	initLogger(level)
}

// SetLogLevelFromFlag는 명령줄 플래그를 파싱하고 로그 레벨을 설정합니다.
// 테스트 코드의 TestMain 함수에서 호출하여 플래그를 파싱하고 로그 레벨을 설정할 수 있습니다.
func SetLogLevelFromFlag() {
	// 플래그가 파싱되지 않은 경우 파싱 시도
	if !flag.Parsed() {
		flag.Parse()
	}

	// 로그 레벨 파싱
	if logLevel != "" {
		var level zapcore.Level
		switch strings.ToUpper(logLevel) {
		case "DEBUG":
			level = zapcore.DebugLevel
		case "INFO":
			level = zapcore.InfoLevel
		case "WARN":
			level = zapcore.WarnLevel
		case "ERROR":
			level = zapcore.ErrorLevel
		default:
			level = DefaultLogLevel
		}
		initLogger(level)
	}
}
