package common

// Diff는 문서 변경 사항을 나타냅니다.
type Diff struct {
	ID         string      `json:"id" bson:"id"`
	Collection string      `json:"collection" bson:"collection"`
	IsNew      bool        `json:"is_new" bson:"is_new"`
	HasChanges bool        `json:"has_changes" bson:"has_changes"`
	Version    int         `json:"version" bson:"version"`
	MergePatch interface{} `json:"merge_patch" bson:"merge_patch"`
	BsonPatch  interface{} `json:"bson_patch" bson:"bson_patch"`
}
