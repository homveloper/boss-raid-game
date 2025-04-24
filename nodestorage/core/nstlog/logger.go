package nstlog

import (
	"os"
	"runtime"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// 기본 로거 인스턴스
	logger *zap.Logger
	// 로거 초기화를 위한 뮤텍스
	loggerMu sync.RWMutex
	// 함수 위치 표시 여부
	showCaller bool
)

// 로거 초기화 함수
func init() {
	// 기본 로거 설정
	SetLogger(true, "info")
}

// SetLogger는 로거를 설정합니다.
// showCallerInfo: 함수 위치 표시 여부
// logLevel: 로그 레벨 (debug, info, warn, error, dpanic, panic, fatal)
func SetLogger(showCallerInfo bool, logLevel string) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	// 로그 레벨 설정
	var level zapcore.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "dpanic":
		level = zapcore.DPanicLevel
	case "panic":
		level = zapcore.PanicLevel
	case "fatal":
		level = zapcore.FatalLevel
	default:
		level = zapcore.InfoLevel
	}

	// 인코더 설정
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 함수 위치 표시 여부 설정
	showCaller = showCallerInfo
	if showCallerInfo {
		encoderConfig.FunctionKey = "func"
	}

	// 코어 설정
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		level,
	)

	// 로거 생성
	logger = zap.New(core)
	if showCallerInfo {
		logger = logger.WithOptions(zap.AddCaller(), zap.AddCallerSkip(1))
	}
}

// GetLogger는 현재 로거 인스턴스를 반환합니다.
func GetLogger() *zap.Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return logger
}

// Debug 로그 메시지를 출력합니다.
func Debug(msg string, fields ...zap.Field) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	l.Debug(msg, fields...)
}

// Info 로그 메시지를 출력합니다.
func Info(msg string, fields ...zap.Field) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	l.Info(msg, fields...)
}

// Warn 로그 메시지를 출력합니다.
func Warn(msg string, fields ...zap.Field) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	l.Warn(msg, fields...)
}

// Error 로그 메시지를 출력합니다.
func Error(msg string, fields ...zap.Field) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	l.Error(msg, fields...)
}

// DPanic 로그 메시지를 출력합니다. (개발 모드에서만 패닉)
func DPanic(msg string, fields ...zap.Field) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	l.DPanic(msg, fields...)
}

// Panic 로그 메시지를 출력하고 패닉을 발생시킵니다.
func Panic(msg string, fields ...zap.Field) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	l.Panic(msg, fields...)
}

// Fatal 로그 메시지를 출력하고 프로그램을 종료합니다.
func Fatal(msg string, fields ...zap.Field) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	l.Fatal(msg, fields...)
}

func Fatalf(format string, args ...interface{}) {
	loggerMu.RLock()
	l := logger
	loggerMu.RUnlock()

	l.Sugar().Fatalf(format, args...)
}

// getCallerFunctionName은 호출자의 함수 이름을 반환합니다.
func getCallerFunctionName() string {
	// 호출 스택에서 2단계 위의 함수 정보를 가져옴 (1: 현재 함수, 2: 로그 함수, 3: 실제 호출자)
	pc, _, _, ok := runtime.Caller(3)
	if !ok {
		return "unknown"
	}

	// 함수 정보 가져오기
	funcInfo := runtime.FuncForPC(pc)
	if funcInfo == nil {
		return "unknown"
	}

	// 전체 함수 이름 (패키지 경로 포함)
	fullName := funcInfo.Name()

	// 패키지 경로와 함수 이름 분리
	parts := strings.Split(fullName, ".")
	if len(parts) <= 1 {
		return fullName
	}

	// 마지막 부분이 함수 이름
	funcName := parts[len(parts)-1]

	// 패키지 이름 (마지막 부분)
	pkgPath := strings.Join(parts[:len(parts)-1], ".")
	pkgParts := strings.Split(pkgPath, "/")
	pkgName := pkgParts[len(pkgParts)-1]

	// 패키지명.함수명 형태로 반환
	return pkgName + "." + funcName
}
