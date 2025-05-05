package crdtstorage

import (
	"context"
	"fmt"
	"tictactoe/luvjson/crdtpatch"
	"time"

	"github.com/google/uuid"
)

// EditTransaction은 분산 환경에서 트랜잭션을 보장하는 문서 편집 메서드입니다.
// 이 메서드는 분산 락과 트랜잭션 추적을 사용하여 여러 노드 간에 일관성을 보장합니다.
func (d *Document) EditTransaction(ctx context.Context, editFunc EditFunc) *TransactionResult {
	result := &TransactionResult{
		Success:  false,
		Document: d,
	}

	// 이미 진행 중인 트랜잭션이 있는지 확인
	if d.activeTransaction != "" {
		result.Error = fmt.Errorf("another transaction is already in progress: %s", d.activeTransaction)
		return result
	}

	// 분산 락 획득 (필요한 경우)
	var lock DistributedLock
	if d.lockManager != nil {
		// 락 생성
		lock = d.lockManager.GetLock(d.ID, d.SessionID.String())

		// 락 획득 시도
		acquired, err := lock.Acquire(ctx, 30*time.Second)
		if err != nil {
			result.Error = fmt.Errorf("failed to acquire distributed lock: %w", err)
			return result
		}

		if !acquired {
			result.Error = fmt.Errorf("failed to acquire distributed lock: timeout")
			return result
		}

		// 함수 종료 시 락 해제
		defer func() {
			if _, err := lock.Release(ctx); err != nil {
				fmt.Printf("Warning: Failed to release distributed lock: %v\n", err)
			}
		}()
	}

	// 로컬 뮤텍스 획득
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// 트랜잭션 시작
	var transactionID string
	if d.transactionManager != nil {
		var err error
		transactionID, err = d.transactionManager.BeginTransaction(ctx, d.ID, d.SessionID.String())
		if err != nil {
			result.Error = fmt.Errorf("failed to begin transaction: %w", err)
			return result
		}

		// 활성 트랜잭션 설정
		d.activeTransaction = transactionID

		// 함수 종료 시 트랜잭션 정리
		defer func() {
			// 성공 시 커밋, 실패 시 중단
			if result.Success {
				if _, err := d.transactionManager.CommitTransaction(ctx, transactionID); err != nil {
					fmt.Printf("Warning: Failed to commit transaction: %v\n", err)
				}
			} else {
				if _, err := d.transactionManager.AbortTransaction(ctx, transactionID); err != nil {
					fmt.Printf("Warning: Failed to abort transaction: %v\n", err)
				}
			}

			// 활성 트랜잭션 초기화
			d.activeTransaction = ""
		}()
	} else {
		// 트랜잭션 관리자가 없는 경우 임시 ID 생성
		transactionID = uuid.New().String()
	}

	// 트랜잭션 시작 마커 생성
	startMarker := &TransactionMarker{
		ID:         transactionID,
		Type:       "start",
		Timestamp:  time.Now(),
		DocumentID: d.ID,
		SessionID:  d.SessionID.String(),
	}

	// 트랜잭션 시작 마커 패치 생성 및 적용
	startPatch := createTransactionPatch(startMarker)
	// 직접 적용
	if err := startPatch.Apply(d.CRDTDoc); err != nil {
		result.Error = fmt.Errorf("failed to apply transaction start marker: %w", err)
		return result
	}
	// 브로드캐스트
	if err := d.SyncManager.ApplyPatch(ctx, startPatch); err != nil {
		// 브로드캐스트 실패는 로그만 남기고 계속 진행
		fmt.Printf("Warning: Failed to broadcast transaction start marker: %v\n", err)
	}

	// 패치 빌더 초기화
	d.PatchBuilder = crdtpatch.NewPatchBuilder(d.SessionID, d.CRDTDoc.NextTimestamp().Counter)

	// 편집 함수 실행
	if err := editFunc(d.CRDTDoc, d.PatchBuilder); err != nil {
		result.Error = fmt.Errorf("edit function failed: %w", err)

		// 트랜잭션 실패 마커 생성 및 적용
		failMarker := &TransactionMarker{
			ID:         transactionID,
			Type:       "abort",
			Timestamp:  time.Now(),
			DocumentID: d.ID,
			SessionID:  d.SessionID.String(),
		}
		failPatch := createTransactionPatch(failMarker)
		// 직접 적용
		if err := failPatch.Apply(d.CRDTDoc); err != nil {
			fmt.Printf("Warning: Failed to apply transaction abort marker: %v\n", err)
		}
		// 브로드캐스트
		if err := d.SyncManager.ApplyPatch(ctx, failPatch); err != nil {
			// 브로드캐스트 실패는 로그만 남기고 계속 진행
			fmt.Printf("Warning: Failed to broadcast transaction abort marker: %v\n", err)
		}

		return result
	}

	// 변경사항 패치 생성
	patch := d.PatchBuilder.Flush()

	// 트랜잭션 ID를 메타데이터에 추가
	metadata := patch.Metadata()
	metadata["transactionId"] = transactionID
	patch.SetMetadata(metadata)

	// 패치 직접 적용
	if err := patch.Apply(d.CRDTDoc); err != nil {
		result.Error = fmt.Errorf("failed to apply patch: %w", err)

		// 트랜잭션 실패 마커 생성 및 적용
		failMarker := &TransactionMarker{
			ID:         transactionID,
			Type:       "abort",
			Timestamp:  time.Now(),
			DocumentID: d.ID,
			SessionID:  d.SessionID.String(),
		}
		failPatch := createTransactionPatch(failMarker)
		// 실패 마커 적용
		if err := failPatch.Apply(d.CRDTDoc); err != nil {
			fmt.Printf("Warning: Failed to apply transaction abort marker: %v\n", err)
		}
		// 실패 마커 브로드캐스트
		d.SyncManager.ApplyPatch(ctx, failPatch)

		return result
	}

	// 패치 브로드캐스트
	if err := d.SyncManager.ApplyPatch(ctx, patch); err != nil {
		// 브로드캐스트 실패는 로그만 남기고 계속 진행
		fmt.Printf("Warning: Failed to broadcast patch: %v\n", err)
	}

	// 마지막 수정 시간 업데이트
	d.LastModified = time.Now()

	// 버전 증가
	d.Version++

	// 자동 저장 (트랜잭션의 일부로 저장하도록 하여 영구성 보장)
	if d.autoSave {
		if err := d.Save(ctx); err != nil {
			// 저장 실패는 로그만 남기고 편집 작업은 성공으로 처리
			fmt.Printf("Warning: Failed to auto-save document %s after edit: %v\n", d.ID, err)
		}
	}

	// 트랜잭션 종료 마커 생성 및 적용
	endMarker := &TransactionMarker{
		ID:         transactionID,
		Type:       "commit",
		Timestamp:  time.Now(),
		DocumentID: d.ID,
		SessionID:  d.SessionID.String(),
	}
	endPatch := createTransactionPatch(endMarker)
	// 직접 적용
	if err := endPatch.Apply(d.CRDTDoc); err != nil {
		fmt.Printf("Warning: Failed to apply transaction commit marker: %v\n", err)
	}
	// 브로드캐스트
	if err := d.SyncManager.ApplyPatch(ctx, endPatch); err != nil {
		// 브로드캐스트 실패는 로그만 남기고 계속 진행
		fmt.Printf("Warning: Failed to broadcast transaction commit marker: %v\n", err)
	}

	// 변경 콜백 호출
	for _, callback := range d.onChangeCallbacks {
		callback(d, patch)
	}

	// 결과 설정
	result.Success = true
	result.Patch = patch
	result.TransactionID = transactionID

	return result
}

// EditWithOptimisticConcurrency는 낙관적 동시성 제어를 사용하여 문서를 편집합니다.
// 이 메서드는 충돌이 발생하면 재시도합니다.
func (d *Document) EditWithOptimisticConcurrency(ctx context.Context, editFunc EditFunc, maxRetries int) *TransactionResult {
	result := &TransactionResult{
		Success:  false,
		Document: d,
	}

	// 최대 재시도 횟수 설정
	if maxRetries <= 0 {
		maxRetries = 3
	}

	// 재시도 루프
	for i := 0; i < maxRetries; i++ {
		// 현재 문서 버전 확인
		currentVersion := d.Version

		// 편집 작업 수행
		tempResult := d.Edit(ctx, editFunc)
		if !tempResult.Success {
			result.Error = tempResult.Error
			continue
		}

		// 버전 확인 (낙관적 동시성 제어)
		if d.Version == currentVersion+1 {
			// 성공
			result.Success = true
			result.Patch = tempResult.Patch
			return result
		}

		// 충돌 발생, 문서 다시 로드 후 재시도
		fmt.Printf("Optimistic concurrency conflict detected, retrying (%d/%d)...\n", i+1, maxRetries)
		time.Sleep(time.Millisecond * 100 * time.Duration(i+1)) // 지수 백오프
	}

	if result.Error == nil {
		result.Error = fmt.Errorf("max retries exceeded")
	}

	return result
}

// EditWithRetry는 편집 작업을 재시도합니다.
// 이 메서드는 문서 옵션에 따라 적절한 편집 메서드를 선택합니다.
func (d *Document) EditWithRetry(ctx context.Context, editFunc EditFunc) *TransactionResult {
	// 문서 옵션 확인
	options := d.GetOptions()

	// 분산 락이 필요한 경우 EditTransaction 사용
	if options.RequireDistributedLock && d.lockManager != nil {
		return d.EditTransaction(ctx, editFunc)
	}

	// 낙관적 동시성 제어가 활성화된 경우 EditWithOptimisticConcurrency 사용
	if options.OptimisticConcurrency {
		return d.EditWithOptimisticConcurrency(ctx, editFunc, options.MaxTransactionRetries)
	}

	// 기본적으로 일반 Edit 사용
	result := d.Edit(ctx, editFunc)
	return &TransactionResult{
		Success:       result.Success,
		Error:         result.Error,
		Patch:         result.Patch,
		Document:      result.Document,
		TransactionID: uuid.New().String(), // 임시 ID 생성
	}
}

// GetOptions는 문서의 옵션을 반환합니다.
func (d *Document) GetOptions() *DocumentOptions {
	// 기본 옵션 생성
	options := DefaultDocumentOptions()

	// 저장소 옵션은 더 이상 Document에서 직접 접근할 수 없습니다.
	// 대신 문서 자체의 설정을 사용합니다.
	options.AutoSave = d.autoSave
	options.AutoSaveInterval = d.autoSaveInterval

	// 문서 메타데이터에서 옵션 가져오기
	if d.Metadata != nil {
		// 자동 저장 설정
		if autoSave, ok := d.Metadata["autoSave"].(bool); ok {
			options.AutoSave = autoSave
		}

		// 낙관적 동시성 제어 설정
		if optimisticConcurrency, ok := d.Metadata["optimisticConcurrency"].(bool); ok {
			options.OptimisticConcurrency = optimisticConcurrency
		}

		// 분산 락 요구 설정
		if requireDistributedLock, ok := d.Metadata["requireDistributedLock"].(bool); ok {
			options.RequireDistributedLock = requireDistributedLock
		}
	}

	return options
}
