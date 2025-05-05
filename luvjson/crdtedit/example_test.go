package crdtedit_test

import (
	"fmt"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtedit"
)

func Example() {
	// 문서 생성
	doc := crdt.NewDocument(common.NewSessionID())

	// 편집기 생성
	editor := crdtedit.NewDocumentEditor(doc)

	// 문서 편집
	err := editor.Edit(func(ctx *crdtedit.EditContext) error {
		// 루트에 객체 생성 (root 키워드 없이도 가능)
		if err := ctx.CreateObject(""); err != nil {
			return err
		}

		// 사용자 객체 생성 및 속성 설정
		if err := ctx.CreateObject("user"); err != nil {
			return err
		}
		if err := ctx.SetObjectKey("user", "name", "John Doe"); err != nil {
			return err
		}
		if err := ctx.SetObjectKey("user", "age", 30); err != nil {
			return err
		}

		// 아이템 배열 생성 및 요소 추가
		if err := ctx.CreateArray("user.items"); err != nil {
			return err
		}
		if err := ctx.AppendArrayElement("user.items", "Item 1"); err != nil {
			return err
		}
		if err := ctx.AppendArrayElement("user.items", "Item 2"); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// 타입별 에디터 사용
	err = editor.Edit(func(ctx *crdtedit.EditContext) error {
		// 객체 에디터 사용 (root 키워드 없이도 가능)
		userObj, err := ctx.AsObject("user")
		if err != nil {
			return err
		}

		// 객체 속성 설정
		if _, err := userObj.SetKey("email", "john@example.com"); err != nil {
			return err
		}

		// 객체의 모든 키 가져오기
		keys, err := userObj.GetKeys()
		if err != nil {
			return err
		}
		fmt.Println("User keys:", keys)

		// 배열 에디터 사용
		itemsArr, err := ctx.AsArray("user.items")
		if err != nil {
			return err
		}

		// 배열 요소 추가
		if _, err := itemsArr.Append("Item 3"); err != nil {
			return err
		}

		// 배열 길이 가져오기
		length, err := itemsArr.GetLength()
		if err != nil {
			return err
		}
		fmt.Println("Items count:", length)

		// 숫자 에디터 사용
		ageNum, err := ctx.AsNumber("user.age")
		if err != nil {
			return err
		}

		// 숫자 증가
		if _, err := ageNum.Increment(1); err != nil {
			return err
		}

		// 현재 값 가져오기
		currentAge, err := ageNum.GetValue()
		if err != nil {
			return err
		}
		fmt.Println("Current age:", currentAge)

		return nil
	})

	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// JSON으로 변환
	jsonData, err := editor.GetJSON()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Document as JSON:", string(jsonData))

	// Output:
	// User keys: [age email items name]
	// Items count: 3
	// Current age: 31
	// Document as JSON: {"user":{"age":31,"email":"john@example.com","items":["Item 1","Item 2","Item 3"],"name":"John Doe"}}
}
