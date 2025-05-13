package eventsync

import (
	"reflect"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetDocumentID는 문서에서 ID 필드를 추출합니다.
// 이 함수는 구조체에 "_id" 또는 "ID" 필드가 있는지 확인하고 해당 값을 반환합니다.
func GetDocumentID(doc interface{}) reflect.Value {
	if doc == nil {
		return reflect.Value{}
	}

	v := reflect.ValueOf(doc)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	// 먼저 bson 태그가 "_id"인 필드 찾기
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if tag := field.Tag.Get("bson"); tag == "_id" || tag == "_id,omitempty" {
			fieldValue := v.Field(i)
			if fieldValue.Type() == reflect.TypeOf(primitive.ObjectID{}) {
				return fieldValue
			}
		}
	}

	// 다음으로 필드 이름이 "ID"인 필드 찾기
	idField := v.FieldByName("ID")
	if idField.IsValid() && idField.Type() == reflect.TypeOf(primitive.ObjectID{}) {
		return idField
	}

	// 마지막으로 필드 이름이 "_id"인 필드 찾기
	idField = v.FieldByName("_id")
	if idField.IsValid() && idField.Type() == reflect.TypeOf(primitive.ObjectID{}) {
		return idField
	}

	return reflect.Value{}
}
