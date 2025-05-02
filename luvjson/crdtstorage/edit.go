package crdtstorage

import (
	"context"
	"fmt"
	"time"

	"tictactoe/luvjson/crdtpatch"

	"github.com/google/uuid"
)

// Edit은 옵션 패턴을 사용하여 문서를 편집합니다.
// 이 함수는 기본적으로 낙관적 동시성 제어를 사용하며, 옵션을 통해 동작을 조정할 수 있습니다.
// 예시:
//
//	doc.Edit(ctx, func(doc *crdt.Document, pb *crdtpatch.PatchBuilder) error {
//	    // 편집 작업
//	    return nil
//	}, WithMaxRetries(5), WithDistributedLock(false))
func (d *Document) Edit(ctx context.Context, editFunc EditFunc, opts ...EditOption) *TransactionResult {
	// 기본 옵션 설정
	options := DefaultEditOptions()

	// 사용자 옵션 적용
	for _, opt := range opts {
		opt(options)
	}

	// 결과 초기화
	result := &TransactionResult{
		Success:  false,
		Document: d,
	}

	// 분산 락 사용이 필요하고 락 관리자가 있는 경우 EditTransaction 사용
	if options.UseDistributedLock && d.lockManager != nil {
		// 트랜잭션 타임아웃 설정
		if options.TransactionTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, options.TransactionTimeout)
			defer cancel()
		}

		// 분산 트랜잭션 실행
		txResult := d.EditTransaction(ctx, editFunc)

		// 자동 저장 설정에 따라 저장
		if txResult.Success && options.SaveAfterEdit && !d.autoSave {
			if err := d.Save(ctx); err != nil {
				fmt.Printf("Warning: Failed to save document %s after edit: %v\n", d.ID, err)
			}
		}

		return txResult
	}

	// 낙관적 동시성 제어 사용
	// 최대 재시도 횟수 설정
	maxRetries := options.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1 // 최소 1번은 시도
	}

	// 재시도 루프
	for i := 0; i < maxRetries; i++ {
		// 현재 문서 버전 확인
		currentVersion := d.Version

		// 로컬 뮤텍스 획득
		d.mutex.Lock()

		// 편집 함수 실행
		var err error
		var patch *crdtpatch.Patch

		// 패치 빌더 초기화
		d.PatchBuilder = crdtpatch.NewPatchBuilder(d.SessionID, d.CRDTDoc.NextTimestamp().Counter)

		// 편집 작업 수행
		if err = editFunc(d.CRDTDoc, d.PatchBuilder); err != nil {
			d.mutex.Unlock()
			result.Error = fmt.Errorf("edit function failed: %w", err)

			// 첫 번째 시도가 아니면 재시도
			if i > 0 {
				continue
			}
			return result
		}

		// 변경사항 패치 생성
		patch = d.PatchBuilder.Flush()

		// 트랜잭션 ID 생성 및 메타데이터에 추가
		transactionID := uuid.New().String()
		metadata := patch.Metadata()
		metadata["transactionId"] = transactionID
		metadata["optimisticConcurrency"] = true
		metadata["retryCount"] = i
		patch.SetMetadata(metadata)

		// 패치 직접 적용
		if err = patch.Apply(d.CRDTDoc); err != nil {
			d.mutex.Unlock()
			result.Error = fmt.Errorf("failed to apply patch: %w", err)

			// 첫 번째 시도가 아니면 재시도
			if i > 0 {
				continue
			}
			return result
		}

		// 패치 브로드캐스트 (ApplyPatch 메서드는 내부적으로 브로드캐스트도 수행)
		if err = d.SyncManager.ApplyPatch(ctx, patch); err != nil {
			// 브로드캐스트 실패는 로그만 남기고 계속 진행
			fmt.Printf("Warning: Failed to broadcast patch: %v\n", err)
		}

		// 마지막 수정 시간 업데이트
		d.LastModified = time.Now()

		// 버전 증가
		d.Version++

		// 자동 저장 (옵션에 따라)
		if options.SaveAfterEdit && !d.autoSave {
			if err := d.Save(ctx); err != nil {
				fmt.Printf("Warning: Failed to auto-save document %s after edit: %v\n", d.ID, err)
			}
		}

		// 변경 콜백 호출
		for _, callback := range d.onChangeCallbacks {
			callback(d, patch)
		}

		// 뮤텍스 해제
		d.mutex.Unlock()

		// 버전 확인 (낙관적 동시성 제어)
		if d.Version == currentVersion+1 {
			// 성공
			result.Success = true
			result.Patch = patch
			result.TransactionID = transactionID
			return result
		}

		// 충돌 발생, 재시도 준비
		fmt.Printf("Optimistic concurrency conflict detected, retrying (%d/%d)...\n", i+1, maxRetries)

		// 지연 시간 계산
		delay := options.RetryDelay
		if options.ExponentialBackoff {
			delay = delay * time.Duration(i+1)
		}

		// 재시도 전 지연
		time.Sleep(delay)
	}

	// 최대 재시도 횟수 초과
	if result.Error == nil {
		result.Error = fmt.Errorf("max retries exceeded (%d)", options.MaxRetries)
	}

	return result
}
