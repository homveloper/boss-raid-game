package testutil

import (
	"os"
	"testing"
)

// TestMainWithLogLevel은 테스트 실행 시 로그 레벨을 설정하는 TestMain 함수의 예시입니다.
// 각 테스트 패키지의 TestMain 함수에서 이 함수를 호출하여 로그 레벨을 설정할 수 있습니다.
//
// 사용 예시:
//
//	func TestMain(m *testing.M) {
//		testutil.TestMainWithLogLevel(m)
//	}
//
// 테스트 실행 시 로그 레벨 지정 방법:
//
//	go test ./... -loglevel=debug
//	go test ./... -loglevel=info
//	go test ./... -loglevel=warn
//	go test ./... -loglevel=error
func TestMainWithLogLevel(m *testing.M) {
	// 명령줄 플래그 파싱 및 로그 레벨 설정
	SetLogLevelFromFlag()

	// 테스트 실행
	os.Exit(m.Run())
}
